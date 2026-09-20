package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"homeitems/internal/config"
	"homeitems/internal/model"
	"homeitems/internal/pkg/argon2id"
)

// ---- Redis key 规划（设计稿 §3.2，全部会话态只有这三组 + 两个辅助集合）----
//
//	auth:refresh:{sha256(refresh)} → {"user_id","family_id","status"}  TTL 7d
//	auth:deny:{jti}                → 1   TTL = access 剩余有效期（登出进黑名单）
//	auth:fail:{username}           → 失败计数  TTL 10min（≥5 锁定）
//	auth:perm:{user_id}            → ["sys:user:list",...]  权限码缓存（角色变更逐出）
//	auth:family:{family_id}        → SET{refresh哈希}   一次登录签发的一串 token
//	auth:userfam:{user_id}         → SET{family_id}     撤销用户全部会话用
const (
	keyRefresh = "auth:refresh:%s"
	keyDeny    = "auth:deny:%s"
	keyFail    = "auth:fail:%s"
	keyPerm    = "auth:perm:%d"
	keyFamily  = "auth:family:%s"
	keyUserFam = "auth:userfam:%d"
)

// 会话错误：handler 按类型映射 HTTP 状态与业务错误码
var (
	ErrBadCred   = errors.New("账号或密码错误") // 统一文案，防用户名枚举
	ErrLocked    = errors.New("失败次数过多，账号已临时锁定，请 10 分钟后再试")
	ErrDisabled  = errors.New("账号已被禁用")
	ErrSession   = errors.New("登录已过期，请重新登录")             // refresh 无效 / 被撤销
	ErrReuse     = errors.New("检测到凭证被重复使用，已撤销本次登录的全部会话") // 重用检测触发
	ErrWrongPass = errors.New("原密码不正确")
)

const maxLoginFails = 5

type Service struct {
	db  *sql.DB
	rdb *redis.Client
	cfg *config.Config
	tm  *TokenManager
}

func NewService(db *sql.DB, rdb *redis.Client, cfg *config.Config) *Service {
	return &Service{db: db, rdb: rdb, cfg: cfg, tm: NewTokenManager(cfg)}
}

// refreshRecord auth:refresh:{hash} 的值
type refreshRecord struct {
	UserID   uint64 `json:"user_id"`
	FamilyID string `json:"family_id"`
	Status   string `json:"status"` // active | used
}

// ============================ 登录 ============================

// Login 密码登录（设计稿图 2 全流程）：
// 限流 → 查用户 → Argon2id 比对 → 写 Redis 白名单 → 返回双 Token。
func (s *Service) Login(username, password, ip, ua string) (*model.LoginUser, string, string, error) {
	ctx := context.Background()

	// ① 登录限流：账号维度计数（网关层另有 IP 维度，这里不重复）
	if n, _ := s.rdb.Get(ctx, fmt.Sprintf(keyFail, username)).Int(); n >= maxLoginFails {
		s.audit(nil, username, "LOGIN_LOCKED", username, "连续失败次数超限", ip, ua)
		return nil, "", "", ErrLocked
	}

	// ② 查用户
	var (
		id   uint64
		hash string
		stat int8
	)
	err := s.db.QueryRow(
		`SELECT id, password_hash, status FROM sys_user WHERE username = ? AND deleted = 0`, username,
	).Scan(&id, &hash, &stat)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// ③ 防枚举：账号不存在时对固定哑哈希做一次同量级计算，两种失败耗时一致
			argon2id.DummyVerify(password)
			s.failOnce(ctx, username)
			s.audit(nil, username, "LOGIN_FAIL", username, "账号不存在", ip, ua)
			return nil, "", "", ErrBadCred
		}
		return nil, "", "", err
	}

	// ④ Argon2id 比对（PHC 前缀分派，兼容 bcrypt 存量）
	ok, err := argon2id.Verify(password, hash)
	if err != nil || !ok {
		argon2id.DummyVerify(password) // 校验失败也补一次计算，抹平耗时差异
		s.failOnce(ctx, username)
		s.audit(&id, username, "LOGIN_FAIL", username, "密码错误", ip, ua)
		return nil, "", "", ErrBadCred
	}
	if stat != 1 {
		s.audit(&id, username, "LOGIN_FAIL", username, "账号已禁用", ip, ua)
		return nil, "", "", ErrDisabled
	}

	// ⑤ 成功：清零失败计数
	s.rdb.Del(ctx, fmt.Sprintf(keyFail, username))

	// ⑥ 签发双 Token：refresh 是 256bit CSPRNG 随机串，服务端只存 SHA-256 摘要
	refresh, _, err := s.issueRefresh(ctx, id, "") // 登录 = 开新 family
	if err != nil {
		return nil, "", "", err
	}
	jti := randHex(16)
	access, err := s.tm.SignAccess(id, jti)
	if err != nil {
		return nil, "", "", err
	}

	// ⑦ 收尾：last_login_at + 审计
	s.db.Exec(`UPDATE sys_user SET last_login_at = NOW() WHERE id = ?`, id)
	s.audit(&id, username, "LOGIN_OK", username, "登录成功", ip, ua)

	u, err := s.UserInfo(id)
	if err != nil {
		return nil, "", "", err
	}
	return u, access, refresh, nil
}

// failOnce 失败计数 +1；首次写入时设 10 分钟 TTL
func (s *Service) failOnce(ctx context.Context, username string) {
	k := fmt.Sprintf(keyFail, username)
	if n, err := s.rdb.Incr(ctx, k).Result(); err == nil && n == 1 {
		s.rdb.Expire(ctx, k, 10*time.Minute)
	}
}

// issueRefresh 生成新 refresh token（active 态入白名单），并登记 family 归属。
// familyID 传空 = 开新 family（登录时）；轮换时必须传入原 familyID ——
// 同一次登录签发的一串 token 属于同一个 family，重用检测才能"撤销整个 family"。
func (s *Service) issueRefresh(ctx context.Context, userID uint64, familyID string) (token, famID string, err error) {
	token = randHex(32) // 256bit CSPRNG
	if familyID == "" {
		familyID = randHex(16)
	}

	h := sha256Hex(token)
	rec, _ := json.Marshal(refreshRecord{UserID: userID, FamilyID: familyID, Status: "active"})
	if err = s.rdb.Set(ctx, fmt.Sprintf(keyRefresh, h), rec, s.cfg.RefreshTTL).Err(); err != nil {
		return "", "", err
	}
	s.rdb.SAdd(ctx, fmt.Sprintf(keyFamily, familyID), h)
	s.rdb.SAdd(ctx, fmt.Sprintf(keyUserFam, userID), familyID)
	return token, familyID, nil
}

// ============================ 轮换与重用检测 ============================

// Refresh 一次一换（设计稿图 3）：
// active → 作废旧 token（标记 used 保留记录）+ 签发全新对（同 family）；
// used   → 重用检测：判定泄露，撤销整个 family。
func (s *Service) Refresh(refreshToken, ip, ua string) (*model.LoginUser, string, string, error) {
	ctx := context.Background()
	h := sha256Hex(refreshToken)

	raw, err := s.rdb.Get(ctx, fmt.Sprintf(keyRefresh, h)).Bytes()
	if err != nil {
		return nil, "", "", ErrSession
	}
	var rec refreshRecord
	if json.Unmarshal(raw, &rec) != nil {
		return nil, "", "", ErrSession
	}

	// ---- 重用检测：used 的 token 再次到达 = 几乎必然是被盗了 ----
	if rec.Status == "used" {
		s.revokeFamily(ctx, rec.FamilyID)
		s.audit(&rec.UserID, "", "REUSE_DETECTED", rec.FamilyID, "旧 refresh 被再次使用，family 已整体撤销", ip, ua)
		return nil, "", "", ErrReuse
	}

	// 用户状态复查：禁用/删除的账号不允许续期
	var stat int8
	var uname string
	err = s.db.QueryRow(`SELECT username, status FROM sys_user WHERE id = ? AND deleted = 0`, rec.UserID).Scan(&uname, &stat)
	if err != nil || stat != 1 {
		s.revokeFamily(ctx, rec.FamilyID)
		return nil, "", "", ErrSession
	}

	// ---- 轮换：旧 token 标记 used（保留剩余 TTL，靠这条记录才能发现重用）----
	rec.Status = "used"
	if b, err := json.Marshal(rec); err == nil {
		k := fmt.Sprintf(keyRefresh, h)
		if ttl, _ := s.rdb.TTL(ctx, k).Result(); ttl > 0 {
			s.rdb.Set(ctx, k, b, ttl)
		}
	}

	// 签发全新一对（沿原 family 推进 —— 这正是 family 概念的意义：整串 token 一损俱损）
	newRefresh, _, err := s.issueRefresh(ctx, rec.UserID, rec.FamilyID)
	if err != nil {
		return nil, "", "", err
	}
	jti := randHex(16)
	access, err := s.tm.SignAccess(rec.UserID, jti)
	if err != nil {
		return nil, "", "", err
	}

	u, err := s.UserInfo(rec.UserID)
	if err != nil {
		return nil, "", "", err
	}
	return u, access, newRefresh, nil
}

// revokeFamily 撤销整个 token family：白名单 key 全删，family 登记一并清掉。
func (s *Service) revokeFamily(ctx context.Context, familyID string) {
	members, _ := s.rdb.SMembers(ctx, fmt.Sprintf(keyFamily, familyID)).Result()
	for _, h := range members {
		s.rdb.Del(ctx, fmt.Sprintf(keyRefresh, h))
	}
	s.rdb.Del(ctx, fmt.Sprintf(keyFamily, familyID))
}

// RevokeUserTokens 撤销某用户的全部会话（改密/改角色/删除用户时调用，设计稿 §7-6）。
func (s *Service) RevokeUserTokens(userID uint64) {
	ctx := context.Background()
	fams, _ := s.rdb.SMembers(ctx, fmt.Sprintf(keyUserFam, userID)).Result()
	for _, f := range fams {
		s.revokeFamily(ctx, f)
	}
	s.rdb.Del(ctx, fmt.Sprintf(keyUserFam, userID))
}

// ============================ 登出 / 黑名单 ============================

// Logout：jti 进黑名单（TTL = access 剩余有效期）+ 删除 refresh 白名单记录。
func (s *Service) Logout(claims *AccessClaims, refreshToken string) {
	ctx := context.Background()

	// 黑名单 TTL 设为剩余寿命：access 本来 15 分钟内自然过期，到期后黑名单也没必要留着
	if remain := time.Until(time.Unix(claims.ExpiresAt, 0)); remain > 0 {
		s.rdb.Set(ctx, fmt.Sprintf(keyDeny, claims.JTI), 1, remain)
	}
	if refreshToken != "" {
		s.rdb.Del(ctx, fmt.Sprintf(keyRefresh, sha256Hex(refreshToken)))
	}
}

// IsDenied 中间件用：jti 是否在黑名单（已登出 / 被撤销的 access）
func (s *Service) IsDenied(jti string) bool {
	n, _ := s.rdb.Exists(context.Background(), fmt.Sprintf(keyDeny, jti)).Result()
	return n > 0
}

// ============================ 修改密码 ============================

func (s *Service) ChangePassword(userID uint64, oldPw, newPw, ip, ua string) error {
	var hash string
	var uname string
	if err := s.db.QueryRow(`SELECT username, password_hash FROM sys_user WHERE id = ?`, userID).Scan(&uname, &hash); err != nil {
		return err
	}
	ok, err := argon2id.Verify(oldPw, hash)
	if err != nil || !ok {
		return ErrWrongPass
	}
	newHash, err := argon2id.Hash(newPw)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(`UPDATE sys_user SET password_hash = ? WHERE id = ?`, newHash, userID); err != nil {
		return err
	}
	// 改密后撤销该用户已签发的全部 refresh（§3.2 / 图 2 注记）
	s.RevokeUserTokens(userID)
	s.audit(&userID, uname, "PASSWORD_CHANGE", uname, "修改密码，全部会话已撤销", ip, ua)
	return nil
}

// ============================ 用户信息 / 权限码 ============================

// UserInfo 装配 /auth/me 视图：基本信息 + 家庭 + 角色 + 全部权限码。
// 前端动态菜单 / 路由 / 按钮全部消费这一份数据 —— 角色差别从这里开始体现。
func (s *Service) UserInfo(userID uint64) (*model.LoginUser, error) {
	u := &model.LoginUser{Roles: []string{}}
	var email, familyName sql.NullString
	err := s.db.QueryRow(`
		SELECT u.id, u.username, u.nickname, u.email, u.family_id, f.name
		FROM sys_user u LEFT JOIN sys_family f ON f.id = u.family_id
		WHERE u.id = ? AND u.deleted = 0`, userID,
	).Scan(&u.ID, &u.Username, &u.Nickname, &email, &u.FamilyID, &familyName)
	if err != nil {
		return nil, err
	}
	if email.Valid {
		u.Email = &email.String
	}
	if familyName.Valid {
		u.FamilyName = &familyName.String
	}

	rows, err := s.db.Query(
		`SELECT r.code FROM sys_user_role ur JOIN sys_role r ON r.id = ur.role_id WHERE ur.user_id = ?`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var code string
		if rows.Scan(&code) == nil {
			u.Roles = append(u.Roles, code)
		}
	}

	codes, err := s.PermCodes(userID)
	if err != nil {
		return nil, err
	}
	u.PermCodes = codes
	return u, nil
}

// PermCodes 权限码：先查 Redis 缓存，miss 则联表查询后回填（设计稿 §4.1）。
// 角色变更时由管理端逐出（EvictPermCache）。
func (s *Service) PermCodes(userID uint64) ([]string, error) {
	ctx := context.Background()
	ck := fmt.Sprintf(keyPerm, userID)

	if b, err := s.rdb.Get(ctx, ck).Bytes(); err == nil {
		codes := []string{}
		if json.Unmarshal(b, &codes) == nil {
			return codes, nil
		}
	}

	// 联表一次查出该用户全部生效权限码
	codes := []string{}
	rows, err := s.db.Query(`
		SELECT DISTINCT p.code
		FROM sys_user_role ur
		JOIN sys_role_permission rp ON rp.role_id = ur.role_id
		JOIN sys_permission p       ON p.id = rp.permission_id
		WHERE ur.user_id = ? AND p.status = 1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		if rows.Scan(&c) == nil {
			codes = append(codes, c)
		}
	}

	// 空结果也缓存，防止缓存穿透
	if b, err := json.Marshal(codes); err == nil {
		s.rdb.Set(ctx, ck, b, s.cfg.PermCacheTTL)
	}
	return codes, nil
}

// EvictPermCache 逐出权限码缓存（角色 / 权限分配变更时调用）。
// 代价：该用户新请求会回源查一次库；收益：权限变更即时生效，不用等 access 过期。
func (s *Service) EvictPermCache(userID uint64) {
	s.rdb.Del(context.Background(), fmt.Sprintf(keyPerm, userID))
}

// ============================ 工具 ============================

// audit 审计日志落库（安全基线 §7-9：操作人 · IP · UA · 时间 · 目标）
func (s *Service) audit(userID *uint64, username, action, target, detail, ip, ua string) {
	if username == "" {
		username = "-"
	}
	if len(ua) > 500 {
		ua = ua[:500]
	}
	s.db.Exec(`INSERT INTO sys_audit_log (user_id, username, action, target, detail, ip, ua)
	           VALUES (?, ?, ?, ?, ?, ?, ?)`, userID, username, action, target, detail, ip, ua)
}

// AuditLog 暴露给其他模块（admin/biz）复用同一张审计表。
func (s *Service) AuditLog(userID *uint64, username, action, target, detail, ip, ua string) {
	s.audit(userID, username, action, target, detail, ip, ua)
}

func randHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// TM 暴露 TokenManager 给中间件复用（验签）。
func (s *Service) TM() *TokenManager { return s.tm }

// DB 暴露数据库连接给中间件复用（用户 status 复查）。
func (s *Service) DB() *sql.DB { return s.db }

// Package admin 管理模块（与业务模块同服务挂载，路由全部要求登录 + 权限码）：
//
//	admin         —— 所有家庭 / 所有用户 / 审计
//	family_admin  —— 本家庭成员增删改查、移交家庭管理员
//
// 行级范围（全部 / 本家庭）在这里按角色下沉到 WHERE 条件。
package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"homeitems/internal/auth"
	"homeitems/internal/model"
	"homeitems/internal/pkg/argon2id"
)

// ---- 业务错误（handler 映射为 400/403 与友好文案）----
var (
	errFamilyNotEmpty     = errors.New("家庭内还有成员，请先处理成员后再删除家庭")
	errTargetNotMember    = errors.New("目标用户不是该家庭的在职成员")
	errTargetIsAdmin      = errors.New("不能对超管做移交操作")
	errTargetAlreadyAdmin = errors.New("目标用户已经是家庭管理员")
	errNoFamily           = errors.New("当前账号未加入任何家庭")
	errNeedFamily         = errors.New("该角色必须归属一个家庭")
	errOutOfScope         = errors.New("只能操作本家庭的成员")
	errAdminOnly          = errors.New("仅超级管理员可执行该操作")
	errDeleteSelf         = errors.New("不能删除自己")
	errDeleteAdmin        = errors.New("不能删除超级管理员")
	errFamilyHasAdmin     = errors.New("该家庭已有家庭管理员，如需更换请使用移交功能")
	errNotFound           = errors.New("目标不存在")
	errNeedReceiver       = errors.New("该用户名下有物品，必须指定接收人")
	errBadReceiver        = errors.New("接收人必须是同家庭的在职成员，且不能是被删用户本人")
)

type Service struct {
	db   *sql.DB
	auth *auth.Service // 复用审计、权限缓存逐出、会话撤销
}

func NewService(db *sql.DB, a *auth.Service) *Service {
	return &Service{db: db, auth: a}
}

// actor 管理操作者快照（与 biz 包同样的角色三问：是不是 admin / family_admin / 属于哪个家庭）
type actor struct {
	id       uint64
	familyID *uint64
	isAdmin  bool
	isFamAdm bool
}

func (s *Service) actorOf(userID uint64) *actor {
	a := &actor{id: userID}
	rows, err := s.db.Query(`SELECT r.code FROM sys_user_role ur
		JOIN sys_role r ON r.id = ur.role_id WHERE ur.user_id = ?`, userID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var code string
			if rows.Scan(&code) == nil {
				switch code {
				case "admin":
					a.isAdmin = true
				case "family_admin":
					a.isFamAdm = true
				}
			}
		}
	}
	var fid sql.NullInt64
	s.db.QueryRow(`SELECT family_id FROM sys_user WHERE id = ?`, userID).Scan(&fid)
	if fid.Valid {
		v := uint64(fid.Int64)
		a.familyID = &v
	}
	return a
}

// familyHasAdmin 该家庭是否已有家庭管理员（排除指定用户 —— 单管理员模型）。
func (s *Service) familyHasAdmin(familyID, excludeUserID uint64) bool {
	var n int
	s.db.QueryRow(`
		SELECT 1 FROM sys_user_role ur
		JOIN sys_role r ON r.id = ur.role_id AND r.code = 'family_admin'
		JOIN sys_user u ON u.id = ur.user_id
		WHERE u.family_id = ? AND u.deleted = 0 AND ur.user_id != ? LIMIT 1`, familyID, excludeUserID).Scan(&n)
	return n == 1
}

// roleID 角色编码 → id
func (s *Service) roleID(code string) uint64 {
	var id uint64
	s.db.QueryRow(`SELECT id FROM sys_role WHERE code = ?`, code).Scan(&id)
	return id
}

// hasRole 用户是否持有某角色
func (s *Service) hasRole(userID uint64, code string) bool {
	var n int
	s.db.QueryRow(`SELECT 1 FROM sys_user_role ur
		JOIN sys_role r ON r.id = ur.role_id WHERE ur.user_id = ? AND r.code = ?`, userID, code).Scan(&n)
	return n == 1
}

// setUserRole 事务式覆盖用户角色（单角色模型），并撤销其会话、逐出权限缓存。
func (s *Service) setUserRole(userID uint64, roleCode string) error {
	rid := s.roleID(roleCode)
	if rid == 0 {
		return sql.ErrNoRows
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM sys_user_role WHERE user_id = ?`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT INTO sys_user_role (user_id, role_id) VALUES (?, ?)`, userID, rid); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.auth.EvictPermCache(userID)
	s.auth.RevokeUserTokens(userID) // 角色变更：旧会话里的身份作废，强制重新登录
	return nil
}

// ============================ 家庭 ============================

// ListFamilies 家庭列表（admin），带成员数与物品数统计。
func (s *Service) ListFamilies() ([]map[string]any, error) {
	rows, err := s.db.Query(`
		SELECT f.id, f.name, f.remark, f.created_at,
		       (SELECT COUNT(*) FROM sys_user u WHERE u.family_id = f.id AND u.deleted = 0) AS members,
		       (SELECT COUNT(*) FROM biz_item i WHERE i.family_id = f.id AND i.deleted = 0) AS items
		FROM sys_family f WHERE f.deleted = 0 ORDER BY f.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var id uint64
		var name, remark string
		var members, items int64
		var created any
		if rows.Scan(&id, &name, &remark, &created, &members, &items) == nil {
			list = append(list, map[string]any{
				"id": id, "name": name, "remark": remark, "createdAt": created,
				"memberCount": members, "itemCount": items,
			})
		}
	}
	return list, nil
}

// CreateFamily 创建家庭，并可选同时创建家庭管理员（admin 一步到位）。
func (s *Service) CreateFamily(name, remark string) (uint64, error) {
	res, err := s.db.Exec(`INSERT INTO sys_family (name, remark) VALUES (?, ?)`, name, remark)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return uint64(id), nil
}

// UpdateFamily 更新家庭名/备注。
func (s *Service) UpdateFamily(id uint64, name, remark string) error {
	_, err := s.db.Exec(`UPDATE sys_family SET name = ?, remark = ? WHERE id = ? AND deleted = 0`, name, remark, id)
	return err
}

// DeleteFamily 软删除家庭（仍有成员时拒绝，先处理成员再删）。
func (s *Service) DeleteFamily(id uint64) error {
	var members int
	s.db.QueryRow(`SELECT COUNT(*) FROM sys_user WHERE family_id = ? AND deleted = 0`, id).Scan(&members)
	if members > 0 {
		return errFamilyNotEmpty
	}
	_, err := s.db.Exec(`UPDATE sys_family SET deleted = 1 WHERE id = ?`, id)
	return err
}

// TransferAdmin 移交家庭管理员：target 成为 family_admin，原管理员降为 member。
// 操作者 family_admin 只能移交自己家庭；admin 可操作任意家庭。
func (s *Service) TransferAdmin(a *actor, familyID, targetUserID uint64) error {
	// 目标用户必须是该家庭的在职成员
	var tfam uint64
	var tstatus int8
	var tdeleted int8
	err := s.db.QueryRow(`SELECT family_id, status, deleted FROM sys_user WHERE id = ?`, targetUserID).
		Scan(&tfam, &tstatus, &tdeleted)
	if err != nil || tdeleted == 1 || tstatus != 1 || tfam != familyID {
		return errTargetNotMember
	}
	if s.hasRole(targetUserID, "admin") {
		return errTargetIsAdmin
	}

	// 找当前家庭管理员（可能没有 —— admin 直接指定首位管理员）
	var oldAdmin sql.NullInt64
	s.db.QueryRow(`
		SELECT ur.user_id FROM sys_user_role ur
		JOIN sys_role r ON r.id = ur.role_id AND r.code = 'family_admin'
		JOIN sys_user u ON u.id = ur.user_id
		WHERE u.family_id = ? AND u.deleted = 0 LIMIT 1`, familyID).Scan(&oldAdmin)

	if oldAdmin.Valid && uint64(oldAdmin.Int64) == targetUserID {
		return errTargetAlreadyAdmin
	}

	// 目标 → family_admin
	if err := s.setUserRole(targetUserID, "family_admin"); err != nil {
		return err
	}
	// 原管理员 → member（降级）
	if oldAdmin.Valid {
		if err := s.setUserRole(uint64(oldAdmin.Int64), "member"); err != nil {
			return err
		}
	}
	return nil
}

// ============================ 用户 ============================

// escapeLike 转义 LIKE 通配符
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// ListUsers 用户列表：admin 看全部（可按 familyId 过滤）；family_admin 只看本家庭。
// 返回行级过滤由调用者角色决定 —— 家庭模型下的"数据范围"。
func (s *Service) ListUsers(a *actor, familyID *uint64, keyword string, status *int8, page, size int) ([]model.User, int64, error) {
	conds := []string{"u.deleted = 0"}
	args := []any{}

	if a.isAdmin {
		if familyID != nil { // admin 可选过滤
			conds = append(conds, "u.family_id = ?")
			args = append(args, *familyID)
		}
	} else { // family_admin：强制本家庭
		if a.familyID == nil {
			return []model.User{}, 0, nil
		}
		conds = append(conds, "u.family_id = ?")
		args = append(args, *a.familyID)
	}

	if kw := strings.TrimSpace(keyword); kw != "" {
		conds = append(conds, "(u.username LIKE ? OR u.nickname LIKE ?)")
		like := "%" + escapeLike(kw) + "%"
		args = append(args, like, like)
	}
	if status != nil {
		conds = append(conds, "u.status = ?")
		args = append(args, *status)
	}
	where := " WHERE " + strings.Join(conds, " AND ")

	var total int64
	if err := s.db.QueryRow("SELECT COUNT(*) FROM sys_user u"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 角色用相关子查询拼装（列表页一次性带出，免 N+1）
	q := `SELECT u.id, u.username, u.nickname, u.email, u.family_id, f.name, u.status,
	             u.last_login_at, u.created_at,
	             (SELECT GROUP_CONCAT(r.code) FROM sys_user_role ur
	              JOIN sys_role r ON r.id = ur.role_id WHERE ur.user_id = u.id) AS roles
	      FROM sys_user u
	      LEFT JOIN sys_family f ON f.id = u.family_id` +
		where + " ORDER BY u.id LIMIT ? OFFSET ?"
	rows, err := s.db.Query(q, append(args, size, (page-1)*size)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users := []model.User{}
	for rows.Next() {
		var u model.User
		var email, familyName, roles sql.NullString
		var lastLogin sql.NullTime
		if err := rows.Scan(&u.ID, &u.Username, &u.Nickname, &email, &u.FamilyID, &familyName,
			&u.Status, &lastLogin, &u.CreatedAt, &roles); err != nil {
			return nil, 0, err
		}
		if email.Valid {
			u.Email = &email.String
		}
		if familyName.Valid {
			u.FamilyName = &familyName.String
		}
		if lastLogin.Valid {
			t := lastLogin.Time
			u.LastLoginAt = &t
		}
		u.RoleCodes = []string{}
		if roles.Valid && roles.String != "" {
			u.RoleCodes = strings.Split(roles.String, ",")
		}
		users = append(users, u)
	}
	return users, total, nil
}

// CreateUser 创建用户。
// admin：任意家庭、任意角色；family_admin：强制本家庭 + member 角色。
func (s *Service) CreateUser(a *actor, username, password, nickname, email string, familyID *uint64, roleCode string) (uint64, error) {
	hash, err := argon2id.Hash(password)
	if err != nil {
		return 0, err
	}

	if !a.isAdmin {
		// 家庭管理员：家庭锁死自己家、角色锁死 member
		if a.familyID == nil {
			return 0, errNoFamily
		}
		familyID = a.familyID
		roleCode = "member"
	}
	if roleCode == "" {
		roleCode = "member"
	}
	if familyID == nil && roleCode != "admin" {
		return 0, errNeedFamily
	}
	// 单管理员模型：一个家庭只允许一个 family_admin，换人走移交（TransferAdmin）
	if roleCode == "family_admin" && familyID != nil && s.familyHasAdmin(*familyID, 0) {
		return 0, errFamilyHasAdmin
	}

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	res, err := tx.Exec(
		`INSERT INTO sys_user (username, password_hash, nickname, email, family_id) VALUES (?, ?, ?, ?, ?)`,
		username, hash, nickname, email, familyID)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()

	rid := s.roleID(roleCode)
	if rid == 0 {
		return 0, sql.ErrNoRows
	}
	if _, err := tx.Exec(`INSERT INTO sys_user_role (user_id, role_id) VALUES (?, ?)`, id, rid); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return uint64(id), nil
}

// UpdateUser 更新资料 / 启停状态（家庭归属与角色走独立接口）。
// family_admin 仅能操作本家庭成员且不能动 admin。
func (s *Service) UpdateUser(a *actor, targetID uint64, nickname, email string, status *int8) error {
	if !s.canTouchUser(a, targetID) {
		return errOutOfScope
	}
	if _, err := s.db.Exec(
		`UPDATE sys_user SET nickname = ?, email = ? WHERE id = ? AND deleted = 0`, nickname, email, targetID); err != nil {
		return err
	}
	if status != nil {
		if _, err := s.db.Exec(`UPDATE sys_user SET status = ? WHERE id = ?`, *status, targetID); err != nil {
			return err
		}
		if *status == 0 {
			s.auth.RevokeUserTokens(targetID) // 禁用即刻踢下线
		}
	}
	return nil
}

// SetUserRole 修改角色（仅 admin）。
func (s *Service) SetUserRole(a *actor, targetID uint64, roleCode string, familyID *uint64) error {
	if !a.isAdmin {
		return errAdminOnly
	}
	if roleCode == "" {
		roleCode = "member"
	}
	// 家庭管理员必须挂在一个家庭上，否则移交/管理无从谈起
	if roleCode == "family_admin" && familyID == nil {
		return errNeedFamily
	}
	// 单管理员模型：设为 family_admin 前检查目标家庭是否已有管理员
	if roleCode == "family_admin" {
		fid := familyID
		if fid == nil {
			var f sql.NullInt64
			s.db.QueryRow(`SELECT family_id FROM sys_user WHERE id = ?`, targetID).Scan(&f)
			if f.Valid {
				v := uint64(f.Int64)
				fid = &v
			}
		}
		if fid == nil {
			return errNeedFamily
		}
		if s.familyHasAdmin(*fid, targetID) {
			return errFamilyHasAdmin
		}
	}
	if familyID != nil {
		if _, err := s.db.Exec(`UPDATE sys_user SET family_id = ? WHERE id = ? AND deleted = 0`, familyID, targetID); err != nil {
			return err
		}
	}
	return s.setUserRole(targetID, roleCode)
}

// DeleteUser 删除用户（软删）—— 关键动作：名下物品的"责任转移"。
//
//	被删用户 owner 的物品全部转给 receiver（接收人，须同家庭），
//	接收人此后对这些物品拥有与原提交者相同的增删改查权；
//	每条转移写 TRANSFER 历史，物品历史完整保留。
func (s *Service) DeleteUser(a *actor, targetID uint64, receiverID *uint64) (transferred int64, err error) {
	if targetID == a.id {
		return 0, errDeleteSelf
	}
	if !s.canTouchUser(a, targetID) {
		return 0, errOutOfScope
	}
	if s.hasRole(targetID, "admin") {
		return 0, errDeleteAdmin
	}

	var tfam sql.NullInt64
	var tdeleted int8
	s.db.QueryRow(`SELECT family_id, deleted FROM sys_user WHERE id = ?`, targetID).Scan(&tfam, &tdeleted)
	if tdeleted == 1 {
		return 0, errNotFound
	}

	// 统计名下作为 owner 的未删物品
	var own int64
	s.db.QueryRow(`SELECT COUNT(*) FROM biz_item WHERE owner_id = ? AND deleted = 0`, targetID).Scan(&own)

	if own > 0 {
		if receiverID == nil {
			return 0, errNeedReceiver
		}
		// 接收人必须存在、在职、同家庭、且不是被删人自己
		var rfam sql.NullInt64
		var rstatus int8
		var rdeleted int8
		e := s.db.QueryRow(`SELECT family_id, status, deleted FROM sys_user WHERE id = ?`, *receiverID).
			Scan(&rfam, &rstatus, &rdeleted)
		if e != nil || rdeleted == 1 || rstatus != 1 || !rfam.Valid || rfam != tfam || *receiverID == targetID {
			return 0, errBadReceiver
		}

		tx, err := s.db.Begin()
		if err != nil {
			return 0, err
		}
		defer tx.Rollback()

		if _, err := tx.Exec(`UPDATE biz_item SET owner_id = ? WHERE owner_id = ? AND deleted = 0`,
			*receiverID, targetID); err != nil {
			return 0, err
		}
		// 每条转移写 TRANSFER 历史（before/after 记责任人变化）
		var receiverName, targetName string
		tx.QueryRow(`SELECT nickname FROM sys_user WHERE id = ?`, *receiverID).Scan(&receiverName)
		tx.QueryRow(`SELECT nickname FROM sys_user WHERE id = ?`, targetID).Scan(&targetName)
		before, _ := json.Marshal(map[string]any{"owner": targetName})
		after, _ := json.Marshal(map[string]any{"owner": receiverName})
		if _, err := tx.Exec(`
			INSERT INTO biz_item_history (item_id, operator_id, operator_name, action, before_json, after_json)
			SELECT id, ?, ?, 'TRANSFER', ?, ? FROM biz_item WHERE owner_id = ? AND deleted = 0`,
			a.id, operatorName(s, a.id), before, after, *receiverID); err != nil {
			return 0, err
		}
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		transferred = own
	}

	// 软删 + 清角色 + 撤会话 + 逐出缓存
	if _, err := s.db.Exec(`UPDATE sys_user SET deleted = 1, status = 0 WHERE id = ?`, targetID); err != nil {
		return 0, err
	}
	s.db.Exec(`DELETE FROM sys_user_role WHERE user_id = ?`, targetID)
	s.auth.RevokeUserTokens(targetID)
	s.auth.EvictPermCache(targetID)
	return transferred, nil
}

// canTouchUser 管理权判定：admin 可动所有人（admin 除外，调用方自查）；
// family_admin 只能动本家庭成员，且不能动任何 admin。
func (s *Service) canTouchUser(a *actor, targetID uint64) bool {
	if a.isAdmin {
		return true
	}
	if !a.isFamAdm || a.familyID == nil {
		return false
	}
	if s.hasRole(targetID, "admin") {
		return false
	}
	var tfam sql.NullInt64
	if err := s.db.QueryRow(`SELECT family_id FROM sys_user WHERE id = ? AND deleted = 0`, targetID).Scan(&tfam); err != nil || !tfam.Valid {
		return false
	}
	return uint64(tfam.Int64) == *a.familyID
}

func operatorName(s *Service, uid uint64) string {
	var nk string
	s.db.QueryRow(`SELECT nickname FROM sys_user WHERE id = ?`, uid).Scan(&nk)
	return nk
}

// FamilyMembers 家庭成员列表（创建用户/移交/删除选接收人时下拉用）。
// family_admin 只能看自己家庭；admin 可指定任意家庭。
func (s *Service) FamilyMembers(a *actor, familyID *uint64) ([]map[string]any, error) {
	fid := familyID
	if !a.isAdmin {
		if a.familyID == nil {
			return []map[string]any{}, nil
		}
		fid = a.familyID
	}
	if fid == nil {
		return []map[string]any{}, nil
	}
	rows, err := s.db.Query(`
		SELECT u.id, u.username, u.nickname,
		       (SELECT GROUP_CONCAT(r.code) FROM sys_user_role ur
		        JOIN sys_role r ON r.id = ur.role_id WHERE ur.user_id = u.id) AS roles
		FROM sys_user u WHERE u.family_id = ? AND u.deleted = 0 ORDER BY u.id`, *fid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []map[string]any{}
	for rows.Next() {
		var id uint64
		var username, nickname string
		var roles sql.NullString
		if rows.Scan(&id, &username, &nickname, &roles) == nil {
			rs := []string{}
			if roles.Valid && roles.String != "" {
				rs = strings.Split(roles.String, ",")
			}
			list = append(list, map[string]any{"id": id, "username": username, "nickname": nickname, "roles": rs})
		}
	}
	return list, nil
}

// ============================ 审计 ============================

// ListAuditLogs 审计日志分页（admin 全量；family_admin 限本家庭成员的操作）。
func (s *Service) ListAuditLogs(a *actor, action string, page, size int) ([]model.AuditLog, int64, error) {
	conds := []string{"1 = 1"}
	args := []any{}

	if !a.isAdmin {
		if a.familyID == nil {
			return []model.AuditLog{}, 0, nil
		}
		// 本家庭成员（含已删）的记录
		conds = append(conds, `user_id IN (SELECT id FROM sys_user WHERE family_id = ?)`)
		args = append(args, *a.familyID)
	}
	if action != "" {
		conds = append(conds, "action = ?")
		args = append(args, action)
	}
	where := " WHERE " + strings.Join(conds, " AND ")

	var total int64
	if err := s.db.QueryRow("SELECT COUNT(*) FROM sys_audit_log"+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	q := `SELECT id, user_id, username, action, target, detail, ip, ua, created_at
	      FROM sys_audit_log` + where + " ORDER BY id DESC LIMIT ? OFFSET ?"
	rows, err := s.db.Query(q, append(args, size, (page-1)*size)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	logs := []model.AuditLog{}
	for rows.Next() {
		var l model.AuditLog
		if err := rows.Scan(&l.ID, &l.UserID, &l.Username, &l.Action, &l.Target,
			&l.Detail, &l.IP, &l.UA, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, total, nil
}

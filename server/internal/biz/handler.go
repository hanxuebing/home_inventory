// Package biz 业务模块：家庭物品整理收纳。
//
// 行级可见性与管理权（全部在 handler 层校验，前端隐藏只是体验）：
//
//	可见范围：物品按 family_id 隔离 —— 家庭内全员可见；admin 可看全部家庭
//	管理权限：owner 本人 / 本家庭的 family_admin / admin 可改可删；
//	          member 对非本人提交（owner != 自己）的物品只读
//	修改历史：CREATE / UPDATE / DELETE / TRANSFER 全部落 biz_item_history
package biz

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"homeitems/internal/auth"
	"homeitems/internal/middleware"
	"homeitems/internal/model"
	"homeitems/internal/pkg/resp"
)

type Handler struct {
	db    *sql.DB
	authS *auth.Service
	dir   string
	maxB  int64
}

func NewHandler(db *sql.DB, a *auth.Service, dir string, maxMB int64) *Handler {
	return &Handler{db: db, authS: a, dir: dir, maxB: maxMB * 1024 * 1024}
}

// ---- 会话工具 ----

func uid(c *gin.Context) uint64 { return c.MustGet("userID").(uint64) }

// actor 当前用户角色能力快照（一次查询，后续判断零成本）
type actor struct {
	id       uint64
	familyID *uint64 // admin 无家庭
	isAdmin  bool
	isFamAdm bool
}

func (h *Handler) actorOf(c *gin.Context) *actor {
	a := &actor{id: uid(c)}
	rows, err := h.db.Query(`SELECT r.code FROM sys_user_role ur
		JOIN sys_role r ON r.id = ur.role_id WHERE ur.user_id = ?`, a.id)
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
	h.db.QueryRow(`SELECT family_id FROM sys_user WHERE id = ?`, a.id).Scan(&fid)
	if fid.Valid {
		v := uint64(fid.Int64)
		a.familyID = &v
	}
	return a
}

func pageOf(c *gin.Context) (page, size int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ = strconv.Atoi(c.DefaultQuery("size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return
}

// escapeLike 转义 LIKE 通配符，让用户输入的 % _ \ 按字面量匹配
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// canManage 管理权判定：owner 本人 / 本家庭 family_admin / admin
func (a *actor) canManage(it *model.Item) bool {
	if a.isAdmin {
		return true
	}
	if a.isFamAdm && a.familyID != nil && it.FamilyID == *a.familyID {
		return true
	}
	return it.OwnerID == a.id
}

// ============================ 物品 ============================

const itemCols = `i.id, i.family_id, i.name, i.quantity, i.category_id, cat.name,
	  i.image, i.remark, i.creator_id, cu.nickname, i.owner_id, ow.nickname,
	  i.created_at, i.updated_at`

func scanItem(rows *sql.Rows) (*model.Item, error) {
	var it model.Item
	var catID sql.NullInt64
	var catName, img sql.NullString
	if err := rows.Scan(&it.ID, &it.FamilyID, &it.Name, &it.Quantity, &catID, &catName,
		&img, &it.Remark, &it.CreatorID, &it.CreatorName, &it.OwnerID, &it.OwnerName,
		&it.CreatedAt, &it.UpdatedAt); err != nil {
		return nil, err
	}
	if catID.Valid {
		v := uint64(catID.Int64)
		it.CategoryID = &v
	}
	if catName.Valid {
		it.CategoryName = &catName.String
	}
	if img.Valid {
		it.Image = &img.String
	}
	return &it, nil
}

// ListItems GET /api/v1/items?keyword=&categoryId=&familyId=&page=&size=
// 模糊查询（名称 / 备注）+ 分类筛选；普通用户固定只看本家庭，admin 可传 familyId 看任意家庭。
func (h *Handler) ListItems(c *gin.Context) {
	a := h.actorOf(c)

	conds := []string{"i.deleted = 0"}
	args := []any{}

	switch {
	case a.isAdmin && c.Query("familyId") != "": // admin 按家庭过滤
		fid, _ := strconv.ParseUint(c.Query("familyId"), 10, 64)
		conds = append(conds, "i.family_id = ?")
		args = append(args, fid)
	case a.isAdmin: // admin 不传 = 看全部
	default: // 家庭用户：固定本家庭
		if a.familyID == nil {
			resp.OK(c, resp.Page{List: []any{}, Total: 0})
			return
		}
		conds = append(conds, "i.family_id = ?")
		args = append(args, *a.familyID)
	}

	if kw := strings.TrimSpace(c.Query("keyword")); kw != "" {
		conds = append(conds, "(i.name LIKE ? OR i.remark LIKE ?)")
		like := "%" + escapeLike(kw) + "%"
		args = append(args, like, like)
	}
	if cid := c.Query("categoryId"); cid != "" {
		if id, err := strconv.ParseUint(cid, 10, 64); err == nil {
			conds = append(conds, "i.category_id = ?")
			args = append(args, id)
		}
	}
	where := " WHERE " + strings.Join(conds, " AND ")
	page, size := pageOf(c)

	var total int64
	if err := h.db.QueryRow("SELECT COUNT(*) FROM biz_item i"+where, args...).Scan(&total); err != nil {
		resp.Internal(c, "查询失败")
		return
	}

	// 名称命中走 idx_family_name 头部；量大了再考虑 FULLTEXT / ES（学习路线后面的章节）
	q := `SELECT ` + itemCols + `
	      FROM biz_item i
	      LEFT JOIN biz_category cat ON cat.id = i.category_id
	      LEFT JOIN sys_user cu ON cu.id = i.creator_id
	      LEFT JOIN sys_user ow ON ow.id = i.owner_id` +
		where + " ORDER BY i.updated_at DESC, i.id DESC LIMIT ? OFFSET ?"
	rows, err := h.db.Query(q, append(args, size, (page-1)*size)...)
	if err != nil {
		resp.Internal(c, "查询失败")
		return
	}
	defer rows.Close()

	items := []*model.Item{}
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			resp.Internal(c, "查询失败")
			return
		}
		items = append(items, it)
	}
	resp.OK(c, resp.Page{List: items, Total: total, Page: page, Size: size})
}

// GetItem GET /api/v1/items/:id —— 详情（家庭内可见）
func (h *Handler) GetItem(c *gin.Context) {
	it, ok := h.loadItem(c)
	if !ok {
		return
	}
	a := h.actorOf(c)
	if !a.isAdmin && (a.familyID == nil || it.FamilyID != *a.familyID) {
		resp.Forbidden(c, "无权查看该物品")
		return
	}
	resp.OK(c, it)
}

// GetItemHistory GET /api/v1/items/:id/history —— 修改历史（家庭内可见）
func (h *Handler) GetItemHistory(c *gin.Context) {
	it, ok := h.loadItem(c)
	if !ok {
		return
	}
	a := h.actorOf(c)
	if !a.isAdmin && (a.familyID == nil || it.FamilyID != *a.familyID) {
		resp.Forbidden(c, "无权查看该物品")
		return
	}
	rows, err := h.db.Query(`
		SELECT h.id, h.item_id, i.name, h.operator_id, h.operator_name,
		       h.action, h.before_json, h.after_json, h.created_at
		FROM biz_item_history h JOIN biz_item i ON i.id = h.item_id
		WHERE h.item_id = ? ORDER BY h.id DESC`, it.ID)
	if err != nil {
		resp.Internal(c, "查询失败")
		return
	}
	defer rows.Close()
	hist := []model.ItemHistory{}
	for rows.Next() {
		var r model.ItemHistory
		if err := rows.Scan(&r.ID, &r.ItemID, &r.ItemName, &r.OperatorID, &r.OperatorName,
			&r.Action, &r.BeforeJSON, &r.AfterJSON, &r.CreatedAt); err == nil {
			hist = append(hist, r)
		}
	}
	resp.OK(c, hist)
}

// loadItem 公共装载：按 id 取未删除物品，失败已写响应
func (h *Handler) loadItem(c *gin.Context) (*model.Item, bool) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	rows, err := h.db.Query(`SELECT `+itemCols+`
		FROM biz_item i
		LEFT JOIN biz_category cat ON cat.id = i.category_id
		LEFT JOIN sys_user cu ON cu.id = i.creator_id
		LEFT JOIN sys_user ow ON ow.id = i.owner_id
		WHERE i.id = ? AND i.deleted = 0`, id)
	if err != nil {
		resp.Internal(c, "查询失败")
		return nil, false
	}
	defer rows.Close()
	if !rows.Next() {
		resp.BadRequest(c, "物品不存在")
		return nil, false
	}
	it, err := scanItem(rows)
	if err != nil {
		resp.Internal(c, "查询失败")
		return nil, false
	}
	return it, true
}

type saveItemReq struct {
	Name       string  `json:"name" binding:"required,max=128"`
	Quantity   int     `json:"quantity" binding:"required,min=0"`
	CategoryID *uint64 `json:"categoryId"`
	Image      *string `json:"image"` // 先 POST /uploads 拿到的 URL，可空
	Remark     string  `json:"remark" binding:"max=1024"`

	// 以下两项仅 admin 代提交时生效：admin 自己没有家庭，
	// 提交物品必须指定"进哪个家庭、算哪个成员提交的"（creator = owner = 该成员）
	FamilyID *uint64 `json:"familyId"`
	OwnerID  *uint64 `json:"ownerId"`
}

// snapshot 物品业务字段的快照（修改历史用）
func snapshot(name string, qty int, image *string, remark string) []byte {
	b, _ := json.Marshal(map[string]any{"name": name, "quantity": qty, "image": image, "remark": remark})
	return b
}

// CreateItem POST /api/v1/items —— 权限码 biz:item:create
// 普通用户：物品进自己所在的家庭，creator = owner = 自己。
// admin 代提交：必须指定 familyId + ownerId，物品算该成员提交的；
// 操作历史 operator 记录的是真实操作者（admin），责任与展示归该成员。
func (h *Handler) CreateItem(c *gin.Context) {
	a := h.actorOf(c)
	var req saveItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "参数不合法（名称必填 <=128 字，数量 >=0）")
		return
	}

	var familyID, submitterID uint64
	if a.isAdmin {
		if req.FamilyID == nil || req.OwnerID == nil {
			resp.BadRequest(c, "超管提交物品必须指定家庭与家庭成员")
			return
		}
		familyID, submitterID = *req.FamilyID, *req.OwnerID
		// 指定成员必须是该家庭的在职成员
		var mfam uint64
		var mstat int8
		err := h.db.QueryRow(
			`SELECT family_id, status FROM sys_user WHERE id = ? AND deleted = 0`, submitterID,
		).Scan(&mfam, &mstat)
		if err != nil || mstat != 1 || mfam != familyID {
			resp.BadRequest(c, "指定成员必须是该家庭的在职成员")
			return
		}
	} else {
		if a.familyID == nil {
			resp.BadRequest(c, "当前账号未加入任何家庭，无法录入物品")
			return
		}
		familyID, submitterID = *a.familyID, a.id
	}

	if req.CategoryID != nil && !h.ownCategory(familyID, *req.CategoryID) {
		resp.BadRequest(c, "分类不存在")
		return
	}

	res, err := h.db.Exec(
		`INSERT INTO biz_item (family_id, name, quantity, category_id, image, remark, creator_id, owner_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		familyID, req.Name, req.Quantity, req.CategoryID, req.Image, req.Remark, submitterID, submitterID)
	if err != nil {
		resp.Internal(c, "保存失败")
		return
	}
	id, _ := res.LastInsertId()

	// 历史留痕：CREATE（operator 是真实操作者，可能是代提交的 admin）
	h.db.Exec(`INSERT INTO biz_item_history (item_id, operator_id, operator_name, action, after_json)
	           VALUES (?, ?, ?, 'CREATE', ?)`,
		id, a.id, h.nickname(a.id), snapshot(req.Name, req.Quantity, req.Image, req.Remark))

	if a.isAdmin {
		h.audit(c, "ITEM_CREATE", req.Name, "超管代提交物品（提交人："+h.nickname(submitterID)+"）")
	} else {
		h.audit(c, "ITEM_CREATE", req.Name, "录入物品")
	}
	resp.OK(c, gin.H{"id": id})
}

// UpdateItem PUT /api/v1/items/:id —— 权限码 biz:item:update
// member 只能改 owner 是自己的；family_admin 本家庭全权；admin 全权。
func (h *Handler) UpdateItem(c *gin.Context) {
	it, ok := h.loadItem(c)
	if !ok {
		return
	}
	a := h.actorOf(c)
	if !a.canManage(it) {
		resp.Forbidden(c, "只能修改自己负责的物品")
		return
	}

	var req saveItemReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "参数不合法")
		return
	}
	if req.CategoryID != nil && !h.ownCategory(it.FamilyID, *req.CategoryID) {
		resp.BadRequest(c, "分类不存在")
		return
	}

	if _, err := h.db.Exec(
		`UPDATE biz_item SET name = ?, quantity = ?, category_id = ?, image = ?, remark = ?
		 WHERE id = ? AND deleted = 0`,
		req.Name, req.Quantity, req.CategoryID, req.Image, req.Remark, it.ID); err != nil {
		resp.Internal(c, "保存失败")
		return
	}

	// 历史留痕：UPDATE（before / after 各一份快照）
	before := snapshot(it.Name, it.Quantity, it.Image, it.Remark)
	after := snapshot(req.Name, req.Quantity, req.Image, req.Remark)
	h.db.Exec(`INSERT INTO biz_item_history (item_id, operator_id, operator_name, action, before_json, after_json)
	           VALUES (?, ?, ?, 'UPDATE', ?, ?)`, it.ID, a.id, h.nickname(a.id), before, after)

	h.audit(c, "ITEM_UPDATE", it.Name, "编辑物品")
	resp.OK(c, nil)
}

// DeleteItem DELETE /api/v1/items/:id —— 权限码 biz:item:delete（软删除，历史保留）
func (h *Handler) DeleteItem(c *gin.Context) {
	it, ok := h.loadItem(c)
	if !ok {
		return
	}
	a := h.actorOf(c)
	if !a.canManage(it) {
		resp.Forbidden(c, "只能删除自己负责的物品")
		return
	}
	if _, err := h.db.Exec(`UPDATE biz_item SET deleted = 1 WHERE id = ?`, it.ID); err != nil {
		resp.Internal(c, "删除失败")
		return
	}

	// 历史留痕：DELETE（软删，记录删除时的快照）
	h.db.Exec(`INSERT INTO biz_item_history (item_id, operator_id, operator_name, action, before_json)
	           VALUES (?, ?, ?, 'DELETE', ?)`,
		it.ID, a.id, h.nickname(a.id), snapshot(it.Name, it.Quantity, it.Image, it.Remark))

	h.audit(c, "ITEM_DELETE", it.Name, "删除物品（软删除，历史保留）")
	resp.OK(c, nil)
}

func (h *Handler) ownCategory(familyID, catID uint64) bool {
	var n int
	h.db.QueryRow(`SELECT 1 FROM biz_category WHERE id = ? AND family_id = ?`, catID, familyID).Scan(&n)
	return n == 1
}

func (h *Handler) nickname(userID uint64) string {
	var nk string
	h.db.QueryRow(`SELECT nickname FROM sys_user WHERE id = ?`, userID).Scan(&nk)
	return nk
}

// ============================ 分类 ============================

// ListCategories GET /api/v1/categories —— 权限码 biz:category:list
// 家庭级：member 只读自己家庭的；admin 可传 familyId。
func (h *Handler) ListCategories(c *gin.Context) {
	a := h.actorOf(c)

	cond := ""
	var fid uint64
	switch {
	case a.isAdmin && c.Query("familyId") != "":
		fid, _ = strconv.ParseUint(c.Query("familyId"), 10, 64)
		cond = "WHERE family_id = ?"
	case a.isAdmin:
		// admin 不传 = 全部家庭
	default:
		if a.familyID == nil {
			resp.OK(c, []any{})
			return
		}
		fid = *a.familyID
		cond = "WHERE family_id = ?"
	}

	q := `SELECT id, family_id, name, sort, created_at FROM biz_category ` + cond + ` ORDER BY sort, id`
	var rows *sql.Rows
	var err error
	if cond == "" {
		rows, err = h.db.Query(q)
	} else {
		rows, err = h.db.Query(q, fid)
	}
	if err != nil {
		resp.Internal(c, "查询失败")
		return
	}
	defer rows.Close()
	cats := []model.Category{}
	for rows.Next() {
		var cat model.Category
		if rows.Scan(&cat.ID, &cat.FamilyID, &cat.Name, &cat.Sort, &cat.CreatedAt) == nil {
			cats = append(cats, cat)
		}
	}
	resp.OK(c, cats)
}

type saveCategoryReq struct {
	Name     string  `json:"name" binding:"required,max=64"`
	Sort     int     `json:"sort"`
	FamilyID *uint64 `json:"familyId"` // 仅 admin 创建时指定；家庭用户强制本家庭
}

// CreateCategory POST /api/v1/categories —— 权限码 biz:category:create（family_admin / admin）
func (h *Handler) CreateCategory(c *gin.Context) {
	a := h.actorOf(c)
	var req saveCategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "分类名必填（<=64 字）")
		return
	}

	fid := req.FamilyID
	if !a.isAdmin {
		if a.familyID == nil {
			resp.BadRequest(c, "当前账号未加入任何家庭")
			return
		}
		fid = a.familyID // 家庭用户只能建自己家庭的分类
	}
	if fid == nil {
		resp.BadRequest(c, "必须指定家庭")
		return
	}

	res, err := h.db.Exec(`INSERT INTO biz_category (family_id, name, sort) VALUES (?, ?, ?)`,
		*fid, req.Name, req.Sort)
	if err != nil {
		resp.BadRequest(c, "创建失败：分类名可能重复")
		return
	}
	id, _ := res.LastInsertId()
	h.audit(c, "CATEGORY_CREATE", req.Name, "新建分类")
	resp.OK(c, gin.H{"id": id})
}

// UpdateCategory PUT /api/v1/categories/:id —— 权限码 biz:category:update
func (h *Handler) UpdateCategory(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if !h.canManageCategory(c, id) {
		return
	}
	var req saveCategoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "参数不合法")
		return
	}
	res, err := h.db.Exec(`UPDATE biz_category SET name = ?, sort = ? WHERE id = ?`, req.Name, req.Sort, id)
	if err != nil {
		resp.Internal(c, "更新失败")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		resp.BadRequest(c, "分类不存在")
		return
	}
	resp.OK(c, nil)
}

// DeleteCategory DELETE /api/v1/categories/:id —— 权限码 biz:category:delete
// 该分类下的物品不删除，置为"未分类"（category_id = NULL）。
func (h *Handler) DeleteCategory(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if !h.canManageCategory(c, id) {
		return
	}

	tx, err := h.db.Begin()
	if err != nil {
		resp.Internal(c, "删除失败")
		return
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE biz_item SET category_id = NULL WHERE category_id = ?`, id); err != nil {
		resp.Internal(c, "删除失败")
		return
	}
	res, err := tx.Exec(`DELETE FROM biz_category WHERE id = ?`, id)
	if err != nil {
		resp.Internal(c, "删除失败")
		return
	}
	if n, _ := res.RowsAffected(); n == 0 {
		resp.BadRequest(c, "分类不存在")
		return
	}
	if err := tx.Commit(); err != nil {
		resp.Internal(c, "删除失败")
		return
	}
	h.audit(c, "CATEGORY_DELETE", c.Param("id"), "删除分类，物品转为未分类")
	resp.OK(c, nil)
}

// canManageCategory 分类管理权并完成家庭归属校验：
// admin 全部；family_admin 仅本家庭；失败时已写好 403 响应。
func (h *Handler) canManageCategory(c *gin.Context, catID uint64) bool {
	a := h.actorOf(c)
	if a.isAdmin {
		return true
	}
	if !a.isFamAdm || a.familyID == nil {
		resp.Forbidden(c, "无权管理分类")
		return false
	}
	var fam uint64
	if err := h.db.QueryRow(`SELECT family_id FROM biz_category WHERE id = ?`, catID).Scan(&fam); err != nil {
		resp.BadRequest(c, "分类不存在")
		return false
	}
	if fam != *a.familyID {
		resp.Forbidden(c, "只能管理本家庭的分类")
		return false
	}
	return true
}

// ============================ 图片上传 ============================

// 允许的图片扩展名（保存时重新生成随机文件名，用户原名不落地）
var allowedExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}

// Upload POST /api/v1/uploads —— multipart/form-data，字段名 file
// 返回 {url: "/uploads/xxx.jpg"}；前端先上传拿 URL，再随物品 JSON 一起提交。
// 移动端 <input type="file" accept="image/*"> 支持直接拍照。
func (h *Handler) Upload(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		resp.BadRequest(c, "缺少文件字段 file")
		return
	}
	if fh.Size > h.maxB {
		resp.BadRequest(c, fmt.Sprintf("图片不能超过 %dMB", h.maxB/1024/1024))
		return
	}
	ext := strings.ToLower(filepath.Ext(fh.Filename))
	if !allowedExts[ext] {
		resp.BadRequest(c, "只支持 jpg / jpeg / png / webp")
		return
	}

	// 随机文件名：用户上传的文件名不可信（可能含路径/奇怪字符），统一丢弃
	b := make([]byte, 4)
	rand.Read(b)
	name := fmt.Sprintf("%d_%s%s", time.Now().UnixNano(), hex.EncodeToString(b), ext)
	if err := c.SaveUploadedFile(fh, filepath.Join(h.dir, name)); err != nil {
		resp.Internal(c, "保存失败")
		return
	}
	resp.OK(c, gin.H{"url": "/uploads/" + name})
}

// EnsureUploadDir 确保 uploads 目录存在（main 启动时调用一次）。
func (h *Handler) EnsureUploadDir() error { return os.MkdirAll(h.dir, 0o755) }

// ============================ 审计与路由 ============================

func (h *Handler) audit(c *gin.Context, action, target, detail string) {
	u := uid(c)
	h.authS.AuditLog(&u, h.nickname(u), action, target, detail, c.ClientIP(), c.Request.UserAgent())
}

// RegisterRoutes 挂载业务路由（jwtAuth 由 main 传入）。
func (h *Handler) RegisterRoutes(r *gin.RouterGroup, jwtAuth gin.HandlerFunc) {
	g := r.Group("", jwtAuth)

	g.GET("/items", middleware.RequirePermission("biz:item:list"), h.ListItems)
	g.GET("/items/:id", middleware.RequirePermission("biz:item:list"), h.GetItem)
	g.GET("/items/:id/history", middleware.RequirePermission("biz:item:list"), h.GetItemHistory)
	g.POST("/items", middleware.RequirePermission("biz:item:create"), h.CreateItem)
	g.PUT("/items/:id", middleware.RequirePermission("biz:item:update"), h.UpdateItem)
	g.DELETE("/items/:id", middleware.RequirePermission("biz:item:delete"), h.DeleteItem)

	g.GET("/categories", middleware.RequirePermission("biz:category:list"), h.ListCategories)
	g.POST("/categories", middleware.RequirePermission("biz:category:create"), h.CreateCategory)
	g.PUT("/categories/:id", middleware.RequirePermission("biz:category:update"), h.UpdateCategory)
	g.DELETE("/categories/:id", middleware.RequirePermission("biz:category:delete"), h.DeleteCategory)

	g.POST("/uploads", middleware.RequirePermission("biz:item:create"), h.Upload)
}

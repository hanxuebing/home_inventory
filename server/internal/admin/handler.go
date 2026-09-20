package admin

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"

	"homeitems/internal/auth"
	"homeitems/internal/middleware"
	"homeitems/internal/pkg/resp"
)

type Handler struct {
	svc   *Service
	authS *auth.Service
}

func NewHandler(svc *Service, authS *auth.Service) *Handler {
	return &Handler{svc: svc, authS: authS}
}

func uid(c *gin.Context) uint64 { return c.MustGet("userID").(uint64) }

func pageOf(c *gin.Context) (page, size int) {
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ = strconv.Atoi(c.DefaultQuery("size", "10"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 10
	}
	return
}

// badReq 统一把 service 层错误映射为响应：越权类 403，其余 400 + 文案。
func badResp(c *gin.Context, err error) {
	if errors.Is(err, errOutOfScope) || errors.Is(err, errAdminOnly) {
		resp.Forbidden(c, err.Error())
		return
	}
	resp.BadRequest(c, err.Error())
}

func (h *Handler) audit(c *gin.Context, action, target, detail string) {
	u := uid(c)
	var username string
	h.svc.db.QueryRow(`SELECT username FROM sys_user WHERE id = ?`, u).Scan(&username)
	h.authS.AuditLog(&u, username, action, target, detail, c.ClientIP(), c.Request.UserAgent())
}

// ============================ 家庭管理（admin） ============================

// ListFamilies GET /api/v1/admin/families —— 权限码 sys:family:list
func (h *Handler) ListFamilies(c *gin.Context) {
	list, err := h.svc.ListFamilies()
	if err != nil {
		resp.Internal(c, "查询失败")
		return
	}
	resp.OK(c, list)
}

type createFamilyReq struct {
	Name   string `json:"name" binding:"required,max=64"`
	Remark string `json:"remark"`
}

// CreateFamily POST /api/v1/admin/families —— 权限码 sys:family:create
func (h *Handler) CreateFamily(c *gin.Context) {
	var req createFamilyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "家庭名必填（<=64 字）")
		return
	}
	id, err := h.svc.CreateFamily(req.Name, req.Remark)
	if err != nil {
		resp.BadRequest(c, "创建失败：家庭名可能重复")
		return
	}
	h.audit(c, "FAMILY_CREATE", req.Name, "创建家庭")
	resp.OK(c, gin.H{"id": id})
}

// UpdateFamily PUT /api/v1/admin/families/:id —— 权限码 sys:family:update
func (h *Handler) UpdateFamily(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req createFamilyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "参数不合法")
		return
	}
	if err := h.svc.UpdateFamily(id, req.Name, req.Remark); err != nil {
		resp.Internal(c, "更新失败")
		return
	}
	h.audit(c, "FAMILY_UPDATE", req.Name, "更新家庭")
	resp.OK(c, nil)
}

// DeleteFamily DELETE /api/v1/admin/families/:id —— 权限码 sys:family:delete
func (h *Handler) DeleteFamily(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.DeleteFamily(id); err != nil {
		badResp(c, err)
		return
	}
	h.audit(c, "FAMILY_DELETE", c.Param("id"), "删除家庭（软删）")
	resp.OK(c, nil)
}

type transferReq struct {
	TargetUserID uint64 `json:"targetUserId" binding:"required"`
}

// TransferAdmin POST /api/v1/admin/families/:id/transfer —— 权限码 sys:family:transfer
// family_admin 移交本家庭 / admin 直接指定任意家庭的管理员。
func (h *Handler) TransferAdmin(c *gin.Context) {
	fid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req transferReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "参数不合法")
		return
	}
	a := h.svc.actorOf(uid(c))

	// family_admin 只能移交自己家庭
	if !a.isAdmin && (a.familyID == nil || *a.familyID != fid) {
		resp.Forbidden(c, "只能移交自己的家庭")
		return
	}
	if err := h.svc.TransferAdmin(a, fid, req.TargetUserID); err != nil {
		badResp(c, err)
		return
	}
	h.audit(c, "TRANSFER_ADMIN", c.Param("id"), "家庭管理员已移交，双方会话已撤销")
	resp.OK(c, nil)
}

// FamilyMembers GET /api/v1/admin/families/:id/members —— 成员下拉（family_admin 只看本家庭）
func (h *Handler) FamilyMembers(c *gin.Context) {
	a := h.svc.actorOf(uid(c))
	fid, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if !a.isAdmin && (a.familyID == nil || *a.familyID != fid) {
		resp.Forbidden(c, "只能查看本家庭的成员")
		return
	}
	list, err := h.svc.FamilyMembers(a, &fid)
	if err != nil {
		resp.Internal(c, "查询失败")
		return
	}
	resp.OK(c, list)
}

// MyFamilyMembers GET /api/v1/admin/families/members —— family_admin 快捷拉本家庭成员
func (h *Handler) MyFamilyMembers(c *gin.Context) {
	a := h.svc.actorOf(uid(c))
	list, err := h.svc.FamilyMembers(a, nil)
	if err != nil {
		resp.Internal(c, "查询失败")
		return
	}
	resp.OK(c, list)
}

// ============================ 用户管理（admin / family_admin） ============================

type listUsersQuery struct {
	FamilyID *uint64 `form:"familyId"`
	Keyword  string  `form:"keyword"`
	Status   *int8   `form:"status"`
}

// ListUsers GET /api/v1/admin/users —— 权限码 sys:user:list
// admin 全部（可按家庭过滤）；family_admin 服务端强制本家庭。
func (h *Handler) ListUsers(c *gin.Context) {
	var q listUsersQuery
	_ = c.ShouldBindQuery(&q)
	page, size := pageOf(c)

	a := h.svc.actorOf(uid(c))
	users, total, err := h.svc.ListUsers(a, q.FamilyID, q.Keyword, q.Status, page, size)
	if err != nil {
		resp.Internal(c, "查询失败")
		return
	}
	resp.OK(c, resp.Page{List: users, Total: total, Page: page, Size: size})
}

type createUserReq struct {
	Username string  `json:"username" binding:"required,min=3,max=64"`
	Password string  `json:"password" binding:"required,min=10"`
	Nickname string  `json:"nickname" binding:"required"`
	Email    string  `json:"email"`
	FamilyID *uint64 `json:"familyId"` // admin 指定；family_admin 忽略（锁本家庭）
	RoleCode string  `json:"roleCode"` // admin 指定；family_admin 忽略（锁 member）
}

// CreateUser POST /api/v1/admin/users —— 权限码 sys:user:create
func (h *Handler) CreateUser(c *gin.Context) {
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "参数不合法（用户名>=3位，初始密码>=10位，昵称必填）")
		return
	}
	a := h.svc.actorOf(uid(c))
	id, err := h.svc.CreateUser(a, req.Username, req.Password, req.Nickname, req.Email, req.FamilyID, req.RoleCode)
	if err != nil {
		badResp(c, err)
		return
	}
	h.audit(c, "USER_CREATE", req.Username, "创建用户")
	resp.OK(c, gin.H{"id": id})
}

type updateUserReq struct {
	Nickname string `json:"nickname" binding:"required"`
	Email    string `json:"email"`
	Status   *int8  `json:"status"`
}

// UpdateUser PUT /api/v1/admin/users/:id —— 权限码 sys:user:update
func (h *Handler) UpdateUser(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		resp.BadRequest(c, "id 不合法")
		return
	}
	var req updateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "参数不合法")
		return
	}
	if id == uid(c) && req.Status != nil && *req.Status == 0 {
		resp.BadRequest(c, "不能禁用自己")
		return
	}
	a := h.svc.actorOf(uid(c))
	if err := h.svc.UpdateUser(a, id, req.Nickname, req.Email, req.Status); err != nil {
		badResp(c, err)
		return
	}
	h.audit(c, "USER_UPDATE", c.Param("id"), "更新用户资料/状态")
	resp.OK(c, nil)
}

type setRoleReq struct {
	RoleCode string  `json:"roleCode" binding:"required"` // admin / family_admin / member
	FamilyID *uint64 `json:"familyId"`
}

// SetUserRole PUT /api/v1/admin/users/:id/role —— 权限码 sys:user:update（仅 admin）
func (h *Handler) SetUserRole(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req setRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "参数不合法")
		return
	}
	if req.RoleCode != "admin" && req.RoleCode != "family_admin" && req.RoleCode != "member" {
		resp.BadRequest(c, "角色只能是 admin / family_admin / member")
		return
	}
	a := h.svc.actorOf(uid(c))
	if err := h.svc.SetUserRole(a, id, req.RoleCode, req.FamilyID); err != nil {
		badResp(c, err)
		return
	}
	h.audit(c, "USER_ROLE", c.Param("id"), "设置角色为 "+req.RoleCode)
	resp.OK(c, nil)
}

type deleteUserReq struct {
	ReceiverID *uint64 `json:"receiverId"` // 名下有物品时必填
}

// DeleteUser DELETE /api/v1/admin/users/:id —— 权限码 sys:user:delete
// 名下有物品时 body 必须带 receiverId（同家庭成员），物品责任整体移交并写 TRANSFER 历史。
func (h *Handler) DeleteUser(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if id == 0 {
		resp.BadRequest(c, "id 不合法")
		return
	}
	var req deleteUserReq
	_ = c.ShouldBindJSON(&req) // DELETE body 可选

	a := h.svc.actorOf(uid(c))
	n, err := h.svc.DeleteUser(a, id, req.ReceiverID)
	if err != nil {
		badResp(c, err)
		return
	}
	h.audit(c, "USER_DELETE", c.Param("id"), "删除用户（软删），物品责任移交 "+strconv.FormatInt(n, 10)+" 条")
	resp.OK(c, gin.H{"transferred": n})
}

// ============================ 审计 ============================

// ListAuditLogs GET /api/v1/admin/audit-logs —— 权限码 sys:audit:list
// admin 全量；family_admin 仅本家庭成员操作。
func (h *Handler) ListAuditLogs(c *gin.Context) {
	page, size := pageOf(c)
	a := h.svc.actorOf(uid(c))
	logs, total, err := h.svc.ListAuditLogs(a, c.Query("action"), page, size)
	if err != nil {
		resp.Internal(c, "查询失败")
		return
	}
	resp.OK(c, resp.Page{List: logs, Total: total, Page: page, Size: size})
}

// ============================ 路由 ============================

// RegisterRoutes 挂载管理路由（jwtAuth 由 main 传入；权限码逐接口声明）。
func (h *Handler) RegisterRoutes(r *gin.RouterGroup, jwtAuth gin.HandlerFunc) {
	g := r.Group("/admin", jwtAuth)

	// 家庭
	g.GET("/families", middleware.RequirePermission("sys:family:list"), h.ListFamilies)
	g.POST("/families", middleware.RequirePermission("sys:family:create"), h.CreateFamily)
	g.PUT("/families/:id", middleware.RequirePermission("sys:family:update"), h.UpdateFamily)
	g.DELETE("/families/:id", middleware.RequirePermission("sys:family:delete"), h.DeleteFamily)
	g.GET("/families/members", h.MyFamilyMembers) // family_admin 拉本家庭成员（登录即可）
	g.GET("/families/:id/members", middleware.RequirePermission("sys:user:list"), h.FamilyMembers)
	g.POST("/families/:id/transfer", middleware.RequirePermission("sys:family:transfer"), h.TransferAdmin)

	// 用户
	g.GET("/users", middleware.RequirePermission("sys:user:list"), h.ListUsers)
	g.POST("/users", middleware.RequirePermission("sys:user:create"), h.CreateUser)
	g.PUT("/users/:id", middleware.RequirePermission("sys:user:update"), h.UpdateUser)
	g.PUT("/users/:id/role", middleware.RequirePermission("sys:user:update"), h.SetUserRole)
	g.DELETE("/users/:id", middleware.RequirePermission("sys:user:delete"), h.DeleteUser)

	// 审计
	g.GET("/audit-logs", middleware.RequirePermission("sys:audit:list"), h.ListAuditLogs)
}

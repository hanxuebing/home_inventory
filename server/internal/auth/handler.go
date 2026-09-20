package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"homeitems/internal/pkg/resp"
)

// Handler /api/v1/auth/* 路由（biz-api 与 admin-api 都挂载，凭据互通）。
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler { return &Handler{svc: svc} }

// cookieName refresh token 的 Cookie 名。
// httpOnly + SameSite=Lax：JS 读不到（防 XSS 偷）+ 基本防 CSRF（§3.3 / §7-7）。
const cookieName = "refresh_token"

func (h *Handler) setRefreshCookie(c *gin.Context, token string) {
	maxAge := int(h.svc.cfg.RefreshTTL.Seconds())
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(cookieName, token, maxAge, "/api/v1/auth", "", h.svc.cfg.CookieSecure, true)
}

func (h *Handler) clearRefreshCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(cookieName, "", -1, "/api/v1/auth", "", h.svc.cfg.CookieSecure, true)
}

type loginReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login POST /api/v1/auth/login
func (h *Handler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "账号和密码不能为空")
		return
	}
	ip, ua := c.ClientIP(), c.Request.UserAgent()

	u, access, refresh, err := h.svc.Login(req.Username, req.Password, ip, ua)
	switch {
	case errors.Is(err, ErrLocked):
		resp.Err(c, http.StatusTooManyRequests, resp.CodeLocked, err.Error())
		return
	case errors.Is(err, ErrBadCred), errors.Is(err, ErrDisabled):
		// 统一映射为 401 + A0230/A0231 语义，前端引导重试或重登
		resp.Unauthorized(c, err.Error())
		return
	case err != nil:
		resp.Internal(c, "登录失败")
		return
	}

	h.setRefreshCookie(c, refresh)
	resp.OK(c, gin.H{
		"accessToken": access,
		"user":        u,
	})
}

// Refresh POST /api/v1/auth/refresh —— 凭 httpOnly Cookie 轮换。
// 正常轮换返回新对；重用检测触发时整个 family 撤销并要求重新登录。
func (h *Handler) Refresh(c *gin.Context) {
	token, err := c.Cookie(cookieName)
	if err != nil || token == "" {
		resp.Unauthorized(c, ErrSession.Error())
		return
	}
	ip, ua := c.ClientIP(), c.Request.UserAgent()

	u, access, newRefresh, err := h.svc.Refresh(token, ip, ua)
	if err != nil {
		h.clearRefreshCookie(c)
		// 重用检测与其他会话失效都返回 401，前端统一走"清状态 → 跳登录页"
		resp.Unauthorized(c, err.Error())
		return
	}
	h.setRefreshCookie(c, newRefresh)
	resp.OK(c, gin.H{
		"accessToken": access,
		"user":        u,
	})
}

// Logout POST /api/v1/auth/logout —— 需已登录（中间件已注入 claims）。
func (h *Handler) Logout(c *gin.Context) {
	claims := c.MustGet("claims").(*AccessClaims)
	h.svc.Logout(claims, readCookie(c))
	h.clearRefreshCookie(c)
	resp.OK(c, nil)
}

// Me GET /api/v1/auth/me —— 当前用户信息 + 权限码（前端动态路由数据源）。
func (h *Handler) Me(c *gin.Context) {
	uid := c.MustGet("userID").(uint64)
	u, err := h.svc.UserInfo(uid)
	if err != nil {
		resp.Unauthorized(c, ErrSession.Error())
		return
	}
	resp.OK(c, u)
}

type changePwReq struct {
	OldPassword string `json:"oldPassword" binding:"required"`
	NewPassword string `json:"newPassword" binding:"required,min=10"`
}

// ChangePassword PUT /api/v1/auth/password
// 密码策略 §7-10：长度 >= 10。成功后全部会话被撤销，前端需跳登录页。
func (h *Handler) ChangePassword(c *gin.Context) {
	var req changePwReq
	if err := c.ShouldBindJSON(&req); err != nil {
		resp.BadRequest(c, "新密码长度至少 10 位")
		return
	}
	uid := c.MustGet("userID").(uint64)
	err := h.svc.ChangePassword(uid, req.OldPassword, req.NewPassword, c.ClientIP(), c.Request.UserAgent())
	switch {
	case errors.Is(err, ErrWrongPass):
		resp.BadRequest(c, err.Error())
	case err != nil:
		resp.Internal(c, "修改失败")
	default:
		h.clearRefreshCookie(c)
		resp.OK(c, nil)
	}
}

func readCookie(c *gin.Context) string {
	if v, err := c.Cookie(cookieName); err == nil {
		return strings.TrimSpace(v)
	}
	return ""
}

// RegisterRoutes 挂载认证路由（两服务共用）。
func (h *Handler) RegisterRoutes(r *gin.RouterGroup, authRequired gin.HandlerFunc) {
	g := r.Group("/auth")
	g.POST("/login", h.Login)
	g.POST("/refresh", h.Refresh)

	authed := g.Group("", authRequired)
	authed.POST("/logout", h.Logout)
	authed.GET("/me", h.Me)
	authed.PUT("/password", h.ChangePassword)
}

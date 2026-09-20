// Package middleware 鉴权中间件链（设计稿 §4.2）：
// 执行顺序固定为 验签 JWT → 查用户 status → 比对权限码，
// 任一步失败返回 401（未认证）或 403（无权限）。
package middleware

import (
	"slices"
	"strings"

	"github.com/gin-gonic/gin"

	"homeitems/internal/auth"
	"homeitems/internal/pkg/resp"
)

// JwtAuth 验签中间件：
//  1. Bearer token 解析与验签（算法白名单 HS256）
//  2. jti 黑名单检查（登出 / 撤销后的 access 立即失效）
//  3. 用户 status 复查（禁用账号即使 token 没过期也拒绝）
//  4. 注入 userID / claims / permCodes 供后续中间件与 handler 使用
func JwtAuth(svc *auth.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			resp.Unauthorized(c, "缺少 access token")
			return
		}
		claims, err := svc.TM().ParseAccess(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			resp.Unauthorized(c, "access token 无效或已过期")
			return
		}

		// 黑名单：登出时写入，TTL = 该 token 剩余寿命
		if svc.IsDenied(claims.JTI) {
			resp.Unauthorized(c, "token 已被撤销，请重新登录")
			return
		}

		// 用户当前状态复查：比"只信 token"多一次 DB 往返，
		// 换来"禁用账号最多再过一个请求就生效"
		var stat int8
		if err := svc.DB().QueryRow(
			`SELECT status FROM sys_user WHERE id = ? AND deleted = 0`, claims.UserID,
		).Scan(&stat); err != nil || stat != 1 {
			resp.Unauthorized(c, "账号不可用")
			return
		}

		// 权限码注入：走 Redis 缓存（角色变更时逐出），不进 JWT payload（§4.2 反模式 ①）
		codes, err := svc.PermCodes(claims.UserID)
		if err != nil {
			resp.Internal(c, "权限加载失败")
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("claims", claims)
		c.Set("permCodes", codes)
		c.Next()
	}
}

// RequirePermission 权限码中间件工厂：路由处逐个声明所需权限码（设计稿 §4.2 原文落地）。
//
//	r.DELETE("/api/v1/admin/users/:id",
//	    JwtAuth(svc),
//	    RequirePermission("sys:user:delete"),
//	    deleteUser)
func RequirePermission(code string) gin.HandlerFunc {
	return func(c *gin.Context) {
		perms := c.MustGet("permCodes").([]string) // JwtAuth 已注入
		if !slices.Contains(perms, code) {         // Go 1.21+ 标准库
			resp.Forbidden(c, "无权限："+code)
			return
		}
		c.Next()
	}
}

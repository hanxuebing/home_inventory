package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORS 跨域配置：开发期前端跑在 Vite (5173/5174)，后端在 8080/8081。
//
// 关键点：refresh token 走 httpOnly Cookie，跨域请求必须
//   - 前端 axios withCredentials: true
//   - 后端 Access-Control-Allow-Credentials: true
//   - 此时 Allow-Origin 不能是 *，必须精确回显白名单内的 Origin
func CORS(allowedOrigins []string) gin.HandlerFunc {
	allow := func(origin string) bool {
		for _, o := range allowedOrigins {
			if o == "*" || o == origin {
				return true
			}
		}
		return false
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allow(origin) {
			c.Header("Access-Control-Allow-Origin", origin) // 精确回显，不用 *
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			c.Header("Access-Control-Max-Age", "86400")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		if strings.Contains(c.Request.URL.Path, "..") { // 拒绝路径穿越（静态文件防御）
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		c.Next()
	}
}

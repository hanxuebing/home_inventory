// api 唯一后端服务（默认 :8080）：
//
//	/api/v1/auth/*       认证（登录 / 刷新 / 登出 / me / 改密码）
//	/api/v1/items|categories|uploads   家庭物品业务
//	/api/v1/admin/*      管理（家庭 / 用户 / 审计 —— admin 与 family_admin 权限不同）
//	/uploads/*           图片静态服务
//
// 前端是一个移动端 Web（web/，Vite :5173），登录后按角色动态出现管理功能。
//
//	go run ./cmd/api
package main

import (
	"time"

	"github.com/gin-gonic/gin"

	"homeitems/internal/admin"
	"homeitems/internal/auth"
	"homeitems/internal/biz"
	"homeitems/internal/config"
	"homeitems/internal/middleware"
	"homeitems/internal/store"
)

func main() {
	cfg := config.Load()
	db := store.OpenMySQL(cfg.MySQlDSN)
	rdb := store.OpenRedis(cfg)

	authSvc := auth.NewService(db, rdb, cfg)
	authH := auth.NewHandler(authSvc)
	bizH := biz.NewHandler(db, authSvc, cfg.UploadDir, cfg.MaxUploadMB)
	adminSvc := admin.NewService(db, authSvc)
	adminH := admin.NewHandler(adminSvc, authSvc)

	if err := bizH.EnsureUploadDir(); err != nil {
		panic("创建上传目录失败: " + err.Error())
	}
	// 孤儿图片清理：每小时对账一轮，无引用且上传超过 24h 的文件删除（biz/janitor.go）
	bizH.StartJanitor(time.Hour, cfg.UploadGrace)

	r := gin.Default()
	// 开发期前端（Vite :5173）跨域；生产由 Nginx 同域反代，不走这套。
	// refresh 走 httpOnly Cookie：Allow-Credentials true + 精确回显 Origin（不能 *）
	r.Use(middleware.CORS([]string{
		"http://localhost:5173", "http://127.0.0.1:5173",
	}))

	api := r.Group("/api/v1")
	jwtAuth := middleware.JwtAuth(authSvc)

	authH.RegisterRoutes(api, jwtAuth)  // 认证
	bizH.RegisterRoutes(api, jwtAuth)   // 家庭物品业务
	adminH.RegisterRoutes(api, jwtAuth) // 管理接口（角色能力在 handler 内细分）

	r.Static("/uploads", cfg.UploadDir) // 图片静态服务

	r.Run(":" + cfg.Port)
}

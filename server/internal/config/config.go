// Package config 从环境变量（.env）加载配置。
// 两个服务（biz-api / admin-api）共享同一套配置：同一个 MySQL、同一个 Redis、
// 同一个 JWT_SECRET —— 这是双 Token 跨服务互通的前提。
package config

import (
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	MySQlDSN     string
	RedisAddr    string
	RedisPwd     string
	RedisDB      int
	JWTSecret    []byte
	JWTIssuer    string
	JWTAudience  string
	AccessTTL    time.Duration
	RefreshTTL   time.Duration
	CookieSecure bool
	Port         string
	UploadDir    string
	MaxUploadMB  int64
	UploadGrace  time.Duration // 孤儿图片宽限期：无引用且上传超过该时长的才清理
	PermCacheTTL time.Duration
}

// Load 读取 .env（不存在则退回纯环境变量），并做最低限度校验。
func Load() *Config {
	_ = godotenv.Load() // .env 可选；生产环境直接注入真实环境变量

	c := &Config{
		MySQlDSN:     must("MYSQL_DSN"),
		RedisAddr:    get("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPwd:     get("REDIS_PASSWORD", ""),
		RedisDB:      0,
		JWTSecret:    []byte(must("JWT_SECRET")),
		JWTIssuer:    get("JWT_ISSUER", "home-items"),
		JWTAudience:  get("JWT_AUDIENCE", "web"),
		AccessTTL:    dur("ACCESS_TTL", 15*time.Minute),
		RefreshTTL:   dur("REFRESH_TTL", 168*time.Hour),
		CookieSecure: get("COOKIE_SECURE", "false") == "true",
		Port:         get("PORT", "8080"),
		UploadDir:    get("UPLOAD_DIR", "./uploads"),
		MaxUploadMB:  int64Env("MAX_UPLOAD_MB", 5),
		UploadGrace:  dur("UPLOAD_GRACE", 24*time.Hour),
		PermCacheTTL: dur("PERM_CACHE_TTL", 30*time.Minute),
	}

	// 安全基线 §7-2：JWT 签名密钥 >= 256 bit
	if len(c.JWTSecret) < 32 {
		log.Fatal("config: JWT_SECRET 长度必须 >= 32 字节（HS256 要求 >=256 bit）")
	}
	return c
}

func must(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("config: 缺少环境变量 %s", key)
	}
	return v
}

func get(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func dur(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		log.Fatalf("config: %s 不是合法时长（如 15m / 168h）", key)
	}
	return def
}

func int64Env(key string, def int64) int64 {
	if v := os.Getenv(key); v != "" {
		n := 0
		for _, ch := range v {
			if ch < '0' || ch > '9' {
				log.Fatalf("config: %s 不是合法整数", key)
			}
			n = n*10 + int(ch-'0')
		}
		return int64(n)
	}
	return def
}

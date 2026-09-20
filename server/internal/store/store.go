// Package store 持久层连接：MySQL（database/sql + 原生 SQL，不用 ORM，
// 学习场景下每条 SQL 都可见）与 Redis（go-redis v9）。
package store

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/redis/go-redis/v9"

	"homeitems/internal/config"
)

func OpenMySQL(dsn string) *sql.DB {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("store: 打开 MySQL 失败: %v", err)
	}
	// 连接池参数：学习场景小并发，给出的是够用且安全的默认
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	// MySQL 8.4 默认 caching_sha2_password，驱动直接支持，无需额外配置
	if err := db.Ping(); err != nil {
		log.Fatalf("store: 连接 MySQL 失败（隧道通了吗？DSN 密码对吗？）: %v", err)
	}
	fmt.Println("[store] MySQL 已连接")
	return db
}

func OpenRedis(cfg *config.Config) *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPwd, // 服务器 Redis 设了 requirepass（NOAUTH 即密码没带上）
		DB:       cfg.RedisDB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("store: 连接 Redis 失败: %v", err)
	}
	fmt.Println("[store] Redis 已连接")
	return rdb
}

// initdb 数据库初始化工具：读取 sql/init.sql 并在 MySQL 上整体执行。
//
//	go run ./cmd/initdb            # 默认读 ../sql/init.sql（相对 server/ 运行）
//	go run ./cmd/initdb D:\path\to\init.sql
//
// 说明：.env 的 MYSQL_DSN 带库名，但首次执行时库还不存在 —— 这里把库名剥掉、
// 改用 multiStatements 连接（脚本自带 CREATE DATABASE / USE，一个连接跑完）。
package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"

	"database/sql"
)

// dsnStripDB 把 /home_items? 形式的库名剥掉，并强制 multiStatements=true
var dbInDsn = regexp.MustCompile(`^(.*@[^/]*)/[^?]*(\?.*)?$`)

func main() {
	_ = godotenv.Load()

	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "缺少 MYSQL_DSN（检查 server/.env）")
		os.Exit(1)
	}
	m := dbInDsn.FindStringSubmatch(dsn)
	if m == nil {
		fmt.Fprintln(os.Stderr, "MYSQL_DSN 格式不识别")
		os.Exit(1)
	}
	params := m[2]
	if params == "" {
		params = "?multiStatements=true"
	} else if !strings.Contains(params, "multiStatements") {
		params += "&multiStatements=true"
	}
	dsn = m[1] + "/" + params

	scriptPath := "../sql/init.sql"
	if len(os.Args) > 1 {
		scriptPath = os.Args[1]
	}
	sqlBytes, err := os.ReadFile(scriptPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "读取脚本失败: %v\n", err)
		os.Exit(1)
	}

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "连接失败:", err)
		os.Exit(1)
	}
	defer db.Close()

	if _, err := db.Exec(string(sqlBytes)); err != nil {
		fmt.Fprintln(os.Stderr, "执行失败:", err)
		os.Exit(1)
	}

	// 幂等性验证：数一下种子用户
	var users int
	if err := db.QueryRow("SELECT COUNT(*) FROM home_items.sys_user").Scan(&users); err != nil {
		fmt.Fprintln(os.Stderr, "验证失败:", err)
		os.Exit(1)
	}
	fmt.Printf("初始化完成 ✓ home_items.sys_user 共 %d 个账号\n", users)
}

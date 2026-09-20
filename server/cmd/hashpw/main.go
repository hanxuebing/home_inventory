// hashpw 生成 Argon2id PHC 哈希，用于：
//  1. 给 sql/init.sql 的占位符生成真实哈希：
//     go run ./cmd/hashpw Admin@123456
//  2. 直接更新某个账号的密码（UPDATE sys_user SET password_hash='...' WHERE username='xxx'）
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"homeitems/internal/pkg/argon2id"
)

func main() {
	if len(os.Args) > 1 {
		// 命令行参数模式：go run ./cmd/hashpw '密码'
		print(os.Args[1])
		return
	}
	// 交互模式：隐藏在参数后面避免密码进 shell 历史
	fmt.Println("输入要哈希的密码（明文仅用于本次计算，不会保存）：")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	if pw := strings.TrimSpace(line); pw != "" {
		print(pw)
	}
}

func print(password string) {
	// 校验逻辑与 ChangePassword 一致：长度 >= 10
	if len(password) < 10 {
		fmt.Fprintln(os.Stderr, "密码长度至少 10 位")
		os.Exit(1)
	}
	hash, err := argon2id.Hash(password)
	if err != nil {
		fmt.Fprintln(os.Stderr, "哈希失败:", err)
		os.Exit(1)
	}
	fmt.Println(hash)
}

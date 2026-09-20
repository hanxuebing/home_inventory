// Package argon2id 密码哈希：OWASP 首选算法 Argon2id（设计稿 §7 降级阶梯第一档）。
//
// 关键设计：
//  1. 哈希串用 PHC 格式自描述（$argon2id$v=19$m=19456,t=2,p=1$盐$摘要），
//     校验时解析前缀即可区分算法 —— 将来降级/换代（bcrypt 与 Argon2id 共存）不用改表结构。
//  2. Go 的 x/crypto/argon2 是纯实现，零系统依赖（设计稿 Fallback 注记）。
package argon2id

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// OWASP 推荐参数：m=19 MiB, t=2, p=1
const (
	memoryKB = 19456
	time     = 2
	threads  = 1
	saltLen  = 16
	keyLen   = 32
)

// Hash 用 Argon2id 生成 PHC 格式哈希串。
func Hash(password string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, time, memoryKB, threads, keyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memoryKB, time, threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// Verify 校验密码。按 PHC 前缀分派：$argon2id$ 走 Argon2id，
// $2a/$2b/$2y$ 走 bcrypt —— 两代算法可共存（设计稿 §7 Fallback）。
func Verify(password, phc string) (bool, error) {
	switch {
	case strings.HasPrefix(phc, "$argon2id$"):
		return verifyArgon2id(password, phc)
	case strings.HasPrefix(phc, "$2a$"), strings.HasPrefix(phc, "$2b$"), strings.HasPrefix(phc, "$2y$"):
		return bcrypt.CompareHashAndPassword([]byte(phc), []byte(password)) == nil, nil
	default:
		return false, errors.New("未识别的哈希格式")
	}
}

func verifyArgon2id(password, phc string) (bool, error) {
	// $argon2id$v=19$m=19456,t=2,p=1$<salt>$<digest>
	parts := strings.Split(phc, "$")
	if len(parts) != 6 {
		return false, errors.New("PHC 格式不完整")
	}

	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, err
	}

	var m uint32
	var t uint32
	var p uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &m, &t, &p); err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, err
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, err
	}

	// 用哈希串里记录的参数（而不是当前默认参数）重算 —— 保证调参后旧哈希仍可验证
	got := argon2.IDKey([]byte(password), salt, t, m, p, uint32(len(want)))
	// subtle.ConstantTimeCompare：恒定时间比较，防时序侧信道
	return subtle.ConstantTimeCompare(got, want) == 1, nil
}

// DummyVerify 用一个固定哈希做一次完整校验计算。
// 用途（设计稿图 2 注记）：账号不存在时也执行同量级的哈希运算，
// 让"账号不存在"和"密码错误"的响应耗时一致，阻断用户名枚举。
func DummyVerify(password string) {
	_, _ = verifyArgon2id(password,
		"$argon2id$v=19$m=19456,t=2,p=1$ZHVtbXlzYWx0ZHVtbXk$Y3Jvc3NfdGltaW5nX2FsaWduX2R1bW15X2tleV9mb3JfZW51bV9wcm90ZWN0aW9u")
}

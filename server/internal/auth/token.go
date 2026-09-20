// Package auth 认证内核：JWT 签发验签、双 Token 轮换、重用检测、
// 登录限流、权限码缓存。biz-api 与 admin-api 共用（同一 MySQL + Redis + 密钥）。
package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"homeitems/internal/config"
)

var (
	ErrTokenInvalid = errors.New("token 无效")
	ErrTokenExpired = errors.New("token 已过期")
)

// AccessClaims JWT 载荷 —— 刻意保持最小化（设计稿 §3.2）：
// payload 只是 Base64URL 编码、不是加密，任何拿到 token 的人都能读，
// 所以只放标识、不放敏感信息。权限码【不进 payload】（§4.2 反模式 ①）。
type AccessClaims struct {
	UserID    uint64 `json:"sub,string"` // json 里 sub 是字符串
	JTI       string `json:"jti"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
}

func (c *AccessClaims) GetExpirationTime() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.ExpiresAt, 0)), nil
}
func (c *AccessClaims) GetIssuedAt() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.IssuedAt, 0)), nil
}
func (c *AccessClaims) GetNotBefore() (*jwt.NumericDate, error) { return nil, nil }
func (c *AccessClaims) GetIssuer() (string, error)              { return c.Issuer, nil }
func (c *AccessClaims) GetSubject() (string, error)             { return "", nil }
func (c *AccessClaims) GetAudience() (jwt.ClaimStrings, error) {
	return jwt.ClaimStrings{c.Audience}, nil
}

type TokenManager struct {
	secret    []byte
	issuer    string
	audience  string
	accessTTL time.Duration
}

func NewTokenManager(cfg *config.Config) *TokenManager {
	return &TokenManager{
		secret:    cfg.JWTSecret,
		issuer:    cfg.JWTIssuer,
		audience:  cfg.JWTAudience,
		accessTTL: cfg.AccessTTL,
	}
}

// SignAccess 签发 access token（HS256）。
func (tm *TokenManager) SignAccess(userID uint64, jti string) (string, error) {
	now := time.Now()
	claims := &AccessClaims{
		UserID:    userID,
		JTI:       jti,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(tm.accessTTL).Unix(),
		Issuer:    tm.issuer,
		Audience:  tm.audience,
	}
	// 安全基线 §7-2：算法白名单 —— 显式指定 HS256，拒绝 none / 算法混淆攻击
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(tm.secret)
}

// ParseAccess 验签并解析（只接受 HS256 + 指定 issuer/audience）。
func (tm *TokenManager) ParseAccess(tokenStr string) (*AccessClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &AccessClaims{}, func(t *jwt.Token) (any, error) {
		// alg 必须是我们签发用的那一种，防止攻击者换算法（如 none / RS256 公钥混淆）
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok || t.Method.Alg() != "HS256" {
			return nil, ErrTokenInvalid
		}
		return tm.secret, nil
	}, jwt.WithIssuer(tm.issuer), jwt.WithAudience(tm.audience), jwt.WithExpirationRequired())
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrTokenExpired
		}
		return nil, ErrTokenInvalid
	}
	claims, ok := token.Claims.(*AccessClaims)
	if !ok || !token.Valid {
		return nil, ErrTokenInvalid
	}
	return claims, nil
}

// AccessTTL 供 handler 计算 jti 黑名单的剩余有效期。
func (tm *TokenManager) AccessTTL() time.Duration { return tm.accessTTL }

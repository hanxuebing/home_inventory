// Package model 领域模型（与 sql/init.sql 的表一一对应）。
package model

import (
	"encoding/json"
	"time"
)

type Family struct {
	ID        uint64    `json:"id"`
	Name      string    `json:"name"`
	Remark    string    `json:"remark"`
	CreatedAt time.Time `json:"createdAt"`
}

// User 管理端用户视图
type User struct {
	ID           uint64     `json:"id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	Nickname     string     `json:"nickname"`
	Email        *string    `json:"email"`
	FamilyID     *uint64    `json:"familyId"`
	FamilyName   *string    `json:"familyName"`
	RoleCodes    []string   `json:"roleCodes"` // admin / family_admin / member
	Status       int8       `json:"status"`
	LastLoginAt  *time.Time `json:"lastLoginAt"`
	CreatedAt    time.Time  `json:"createdAt"`
}

type Role struct {
	ID     uint64 `json:"id"`
	Code   string `json:"code"`
	Name   string `json:"name"`
	Remark string `json:"remark"`
}

type Permission struct {
	ID        uint64  `json:"id"`
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Type      string  `json:"type"` // MENU / API / BUTTON
	ParentID  *uint64 `json:"parentId"`
	Path      *string `json:"path"`
	Component *string `json:"component"`
	Icon      *string `json:"icon"`
	Sort      int     `json:"sort"`
	Status    int8    `json:"status"`
}

type AuditLog struct {
	ID        uint64    `json:"id"`
	UserID    *uint64   `json:"userId"`
	Username  string    `json:"username"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail"`
	IP        string    `json:"ip"`
	UA        string    `json:"ua"`
	CreatedAt time.Time `json:"createdAt"`
}

type Category struct {
	ID        uint64    `json:"id"`
	FamilyID  uint64    `json:"familyId"`
	Name      string    `json:"name"`
	Sort      int       `json:"sort"`
	CreatedAt time.Time `json:"createdAt"`
}

// Item 家庭物品：家庭内全员可见（按 family_id 过滤），
// creator 是原始提交人（展示"谁录入的"），owner 是当前责任人（管理权随它走）。
type Item struct {
	ID           uint64    `json:"id"`
	FamilyID     uint64    `json:"familyId"`
	Name         string    `json:"name"`
	Quantity     int       `json:"quantity"`
	CategoryID   *uint64   `json:"categoryId"`
	CategoryName *string   `json:"categoryName"`
	Image        *string   `json:"image"` // 如 /uploads/xxx.jpg，可空
	Remark       string    `json:"remark"`
	CreatorID    uint64    `json:"creatorId"`
	CreatorName  string    `json:"creatorName"`
	OwnerID      uint64    `json:"ownerId"`
	OwnerName    string    `json:"ownerName"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

// ItemHistory 修改历史：CREATE / UPDATE / DELETE / TRANSFER 全程留痕
type ItemHistory struct {
	ID           uint64          `json:"id"`
	ItemID       uint64          `json:"itemId"`
	ItemName     string          `json:"itemName"`
	OperatorID   uint64          `json:"operatorId"`
	OperatorName string          `json:"operatorName"`
	Action       string          `json:"action"`
	BeforeJSON   json.RawMessage `json:"before"`
	AfterJSON    json.RawMessage `json:"after"`
	CreatedAt    time.Time       `json:"createdAt"`
}

// LoginUser /auth/me 与 /auth/login 成功后返回的用户视图（前端动态功能的数据源）
type LoginUser struct {
	ID         uint64   `json:"id"`
	Username   string   `json:"username"`
	Nickname   string   `json:"nickname"`
	Email      *string  `json:"email"`
	FamilyID   *uint64  `json:"familyId"` // admin 为 null
	FamilyName *string  `json:"familyName"`
	Roles      []string `json:"roles"` // admin / family_admin / member
	PermCodes  []string `json:"permCodes"`
}

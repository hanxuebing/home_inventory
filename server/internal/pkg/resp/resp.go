// Package resp 统一响应结构 {code, message, data}。
// code = 0 表示成功；非 0 使用 A0xxx 业务错误码（与设计稿 §6 契约一致），
// HTTP 状态码同步返回（401 未认证 / 403 无权限 / 200 成功），前端两处都可用。
package resp

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// 业务错误码（设计稿 §4.2 / §6）
const (
	CodeOK           = 0
	CodeLoginFail    = "A0230" // 账号或密码错误（统一文案，防枚举）
	CodeLocked       = "A0240" // 失败次数过多，暂时锁定
	CodeUnauthorized = "A0231" // 未认证 / token 无效或已过期
	CodeForbidden    = "A0301" // 已认证但无权限
	CodeBadRequest   = "A0402" // 参数错误
	CodeInternal     = "A0500" // 服务器内部错误
)

type Body struct {
	Code    any    `json:"code"` // int(0) 或 string(A0xxx)
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type Page struct {
	List  any   `json:"list"`
	Total int64 `json:"total"`
	Page  int   `json:"page"`
	Size  int   `json:"size"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Body{Code: CodeOK, Message: "ok", Data: data})
}

func Err(c *gin.Context, httpStatus int, code any, msg string) {
	c.AbortWithStatusJSON(httpStatus, Body{Code: code, Message: msg, Data: nil})
}

func Unauthorized(c *gin.Context, msg string) {
	Err(c, http.StatusUnauthorized, CodeUnauthorized, msg)
}

func Forbidden(c *gin.Context, msg string) {
	Err(c, http.StatusForbidden, CodeForbidden, msg)
}

func BadRequest(c *gin.Context, msg string) {
	Err(c, http.StatusBadRequest, CodeBadRequest, msg)
}

func Internal(c *gin.Context, msg string) {
	Err(c, http.StatusInternalServerError, CodeInternal, msg)
}

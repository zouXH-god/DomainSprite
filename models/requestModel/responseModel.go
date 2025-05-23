package requestModel

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

// Response 通用响应结构
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// 常见的响应代码
const (
	SuccessCode      = 200
	ErrorCode        = 500
	BadRequestCode   = 400
	NotFoundCode     = 404
	UnauthorizedCode = 401
)

// Success 生成成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, Response{
		Code:    SuccessCode,
		Message: "Success",
		Data:    data,
	})
}

// Error 生成错误响应
func Error(c *gin.Context, code int, message string, data interface{}) {
	c.JSON(code, Response{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// BadRequest 生成400响应
func BadRequest(c *gin.Context, message string) {
	Error(c, BadRequestCode, message, nil)
}

func BadRequestWithData(c *gin.Context, message string, data interface{}) {
	Error(c, BadRequestCode, message, data)
}

// NotFound 生成404响应
func NotFound(c *gin.Context, message string) {
	Error(c, NotFoundCode, message, nil)
}

// Unauthorized 生成401响应
func Unauthorized(c *gin.Context, message string) {
	Error(c, UnauthorizedCode, message, nil)
}

// Forbidden 生成403响应
func Forbidden(c *gin.Context, message string) {
	Error(c, 403, message, nil)
}

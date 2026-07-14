package response

import "github.com/gin-gonic/gin"

type Body struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

func OK(c *gin.Context, data interface{}) {
	c.JSON(200, Body{Code: 0, Msg: "ok", Data: data})
}

func Fail(c *gin.Context, code int, msg string) {
	c.JSON(200, Body{Code: code, Msg: msg})
}

func FailHTTP(c *gin.Context, httpCode int, code int, msg string) {
	c.JSON(httpCode, Body{Code: code, Msg: msg})
}

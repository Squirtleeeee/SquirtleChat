package handler

import (
	"squirtlechat/pkg/apperr"
	"squirtlechat/pkg/response"

	"github.com/gin-gonic/gin"
)

func fail(c *gin.Context, code int, err error) {
	msg := apperr.ToUserMessage(code, err)
	response.Fail(c, code, msg)
}

func failParam(c *gin.Context, err error) {
	fail(c, apperr.ErrInvalidParam, err)
}

func failUnauthorized(c *gin.Context, err error) {
	fail(c, apperr.ErrUnauthorized, err)
}

func failConflict(c *gin.Context, err error) {
	fail(c, apperr.ErrConflict, err)
}

func failInternal(c *gin.Context, err error) {
	fail(c, apperr.ErrInternal, err)
}

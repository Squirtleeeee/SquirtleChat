package middleware

import (
	"strings"

	"squirtlechat/internal/service"
	"squirtlechat/pkg/apperr"
	"squirtlechat/pkg/response"

	"github.com/gin-gonic/gin"
)

func Auth(authSvc *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if h == "" || !strings.HasPrefix(h, "Bearer ") {
			response.FailHTTP(c, 401, apperr.ErrUnauthorized, apperr.CodeMessage(apperr.ErrUnauthorized))
			c.Abort()
			return
		}
		claims, err := authSvc.ParseToken(strings.TrimPrefix(h, "Bearer "))
		if err != nil {
			response.FailHTTP(c, 401, apperr.ErrUnauthorized, apperr.CodeMessage(apperr.ErrUnauthorized))
			c.Abort()
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("device_id", claims.DeviceID)
		c.Next()
	}
}

func UserID(c *gin.Context) int64 {
	v, _ := c.Get("user_id")
	id, _ := v.(int64)
	return id
}

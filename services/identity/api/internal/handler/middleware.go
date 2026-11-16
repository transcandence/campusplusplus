package handler

import (
	"crypto/rsa"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"github.com/transcandence/campusplusplus/platform/go/auth"
)

// RequestIDMiddleware injects an RFC 4122 UUID into the X-Request-Id header and context.
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-Id")
		if reqID == "" {
			reqID = uuid.NewString()
		}
		c.Header("X-Request-Id", reqID)
		c.Set("requestId", reqID)
		c.Next()
	}
}

// CORSMiddleware handles CORS headers and preflight OPTIONS requests.
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Request-Id")
		c.Header("Access-Control-Expose-Headers", "X-Request-Id, Content-Length")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

// BearerAuthMiddleware validates Bearer JWT access tokens for protected routes.
func BearerAuthMiddleware(publicKey *rsa.PublicKey) MiddlewareFunc {
	return func(c *gin.Context) {
		// Only enforce authentication if BearerAuthScopes was set by generated wrapper
		if _, exists := c.Get(BearerAuthScopes); !exists {
			c.Next()
			return
		}

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
			respondProblem(c, http.StatusUnauthorized, "unauthorized", "Missing or malformed Authorization header")
			c.Abort()
			return
		}

		tokenString := strings.TrimSpace(authHeader[7:])
		claims, err := auth.VerifyAccessToken(tokenString, publicKey)
		if err != nil {
			respondProblem(c, http.StatusUnauthorized, "unauthorized", "Invalid or expired access token")
			c.Abort()
			return
		}

		c.Set("userID", claims.UserID)
		c.Set("sessionID", claims.SessionID)
		c.Set("campusID", claims.CampusID)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// CustomErrorHandler formats framework and binding errors as RFC 9457 Problem Details.
func CustomErrorHandler(c *gin.Context, err error, statusCode int) {
	respondProblem(c, statusCode, "validation_failed", err.Error())
}

func respondProblem(c *gin.Context, status int, code, detail string) {
	reqIDStr := c.GetString("requestId")
	var reqIDUUID *openapi_types.UUID
	if reqIDStr != "" {
		if parsed, err := uuid.Parse(reqIDStr); err == nil {
			reqIDUUID = &parsed
		}
	}

	title := http.StatusText(status)
	instance := c.Request.RequestURI

	c.Header("Content-Type", "application/problem+json")
	c.JSON(status, Problem{
		Type:      "about:blank",
		Title:     title,
		Status:    status,
		Code:      &code,
		Detail:    &detail,
		Instance:  &instance,
		RequestId: reqIDUUID,
	})
}

package handler

import "github.com/gin-gonic/gin"

// Claims is the authorization context populated by the Bearer middleware.
// The middleware only has to set the context keys; reading them is shared
// here so handlers and tests agree on one contract.
type Claims struct {
	UserID     int64
	Role       string
	WorkerRole string
	CampusID   int64
	SessionID  int64
}

// ClaimsFromContext reads the claims set by the auth middleware. It returns
// false when there is no authenticated user (missing token / invalid token).
func ClaimsFromContext(c *gin.Context) (Claims, bool) {
	rawUserID, exists := c.Get("userID")
	if !exists {
		return Claims{}, false
	}
	userID, ok := rawUserID.(int64)
	if !ok || userID == 0 {
		return Claims{}, false
	}

	var claims Claims
	claims.UserID = userID

	if v, ok := c.Get("role"); ok {
		claims.Role, _ = v.(string)
	}
	if v, ok := c.Get("workerRole"); ok {
		claims.WorkerRole, _ = v.(string)
	}
	if v, ok := c.Get("campusID"); ok {
		claims.CampusID, _ = v.(int64)
	}
	if v, ok := c.Get("sessionID"); ok {
		claims.SessionID, _ = v.(int64)
	}

	return claims, true
}

package middleware

import (
	"crypto/rsa"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/transcandence/campusplusplus/platform/go/auth"
)

var logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))

type ProblemDetails struct {
	Type     string `json:"type"`
	Title    string `json:"title"`
	Status   int    `json:"status"`
	Detail   string `json:"detail"`
	Instance string `json:"instance"`
}

func isPublicRoute(Method, Path string) bool {
	switch {
	case Method == http.MethodPost && Path == "/api/v1/identity/auth/login":
		return true
	case Method == http.MethodPost && Path == "/api/v1/identity/auth/refresh":
		return true
	case Method == http.MethodPost && Path == "/api/v1/identity/auth/register":
		return true
	case Method == http.MethodGet && Path == "/healthz":
		return true
	default:
		return false
	}
}

func writeProblem401(w http.ResponseWriter, r *http.Request, detail string) {
	problem := ProblemDetails{
		Type:     "https://datatracker.ietf.org/doc/html/rfc9110#section-15.5.2",
		Title:    "Unauthorized",
		Status:   http.StatusUnauthorized,
		Detail:   detail,
		Instance: r.URL.Path,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(problem)
}

func JwtAuth(publicKey *rsa.PublicKey, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Header.Del("X-User-Id")
		r.Header.Del("X-User-Role")
		r.Header.Del("X-Campus-Id")
		r.Header.Del("X-Session-Id")
		if isPublicRoute(r.Method, r.URL.Path) {
			logger.Info("auth bypassed for public route", "method", r.Method, "path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
			)
			next.ServeHTTP(w, r)
			return
		}
		authHeader := r.Header.Get("Authorization")
		parts := strings.Fields(authHeader)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			logger.Warn("unauthorized request: missing or malformed bearer header",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
			)
			writeProblem401(w, r, "Missing or malformed Authorization header.")
			return
		}
		claims, err := auth.VerifyAccessToken(parts[1], publicKey)
		if err != nil {
			logger.Warn("unauthorized request: token verification failed",
				"method", r.Method,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr,
				"error", err.Error(),
			)
			writeProblem401(w, r, "Access token is invalid or expired.")
			return
		}
		userID := strconv.FormatInt(claims.UserID, 10)
		campusID := strconv.FormatInt(claims.CampusID, 10)
		sessionID := strconv.FormatInt(claims.SessionID, 10)

		r.Header.Set("X-User-Id", userID)
		r.Header.Set("X-User-Role", claims.Role)
		r.Header.Set("X-Campus-Id", campusID)
		r.Header.Set("X-Session-Id", sessionID)

		logger.Info("request authenticated",
			"method", r.Method,
			"path", r.URL.Path,
			"user_id", userID,
			"role", claims.Role,
			"campus_id", campusID,
			"session_id", sessionID,
		)
		next.ServeHTTP(w, r)
	})
}

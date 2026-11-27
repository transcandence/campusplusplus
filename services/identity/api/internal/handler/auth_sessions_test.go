package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/transcandence/campusplusplus/services/identity/api/internal/model"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/service"
)

func newSessionRouter(repo *logoutSessionRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	authService := service.NewAuthService(&fakeUserRepository{}, repo, nil, time.Minute, time.Hour, 10)
	authHandler := NewAuthHandler(authService, nil)

	withIdentity := func(c *gin.Context) {
		c.Set("userID", int64(42))
		c.Set("sessionID", int64(7))
		c.Next()
	}

	router := gin.New()
	router.GET("/sessions", withIdentity, authHandler.ListSessions)
	router.DELETE("/sessions/:sessionId", withIdentity, func(c *gin.Context) {
		sessionID, err := strconv.ParseInt(c.Param("sessionId"), 10, 64)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		authHandler.RevokeSession(c, sessionID)
	})
	return router
}

func TestListSessionsMarksCurrentSession(t *testing.T) {
	repo := &logoutSessionRepository{sessions: []model.SessionSummary{
		{ID: 7, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)},
		{ID: 8, CreatedAt: time.Now(), ExpiresAt: time.Now().Add(time.Hour)},
	}}
	router := newSessionRouter(repo)

	recorder := performJSON(router, http.MethodGet, "/sessions", "")

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var sessions []SessionSummary
	if err := json.Unmarshal(recorder.Body.Bytes(), &sessions); err != nil {
		t.Fatalf("decoding sessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("len(sessions) = %d, want 2", len(sessions))
	}
	if !sessions[0].Current {
		t.Errorf("session 7 should be current")
	}
	if sessions[1].Current {
		t.Errorf("session 8 should not be current")
	}
}

func TestListSessionsRepositoryErrorReturns500(t *testing.T) {
	router := newSessionRouter(&logoutSessionRepository{listErr: errors.New("db down")})

	recorder := performJSON(router, http.MethodGet, "/sessions", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

func TestRevokeSessionSuccess(t *testing.T) {
	repo := &logoutSessionRepository{revokeByUserFound: true}
	router := newSessionRouter(repo)

	recorder := performJSON(router, http.MethodDelete, "/sessions/8", "")

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if repo.revokedForUserID != 42 {
		t.Errorf("revoke user id = %d, want 42", repo.revokedForUserID)
	}
}

func TestRevokeSessionNotOwnedReturns404(t *testing.T) {
	router := newSessionRouter(&logoutSessionRepository{revokeByUserFound: false})

	recorder := performJSON(router, http.MethodDelete, "/sessions/99", "")

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	problem := decodeProblem(t, recorder)
	if problem["code"] != model.ErrCodeSessionNotFound {
		t.Errorf("problem code = %v, want %q", problem["code"], model.ErrCodeSessionNotFound)
	}
}

func TestRevokeSessionRepositoryErrorReturns500(t *testing.T) {
	router := newSessionRouter(&logoutSessionRepository{revokeByUserErr: errors.New("db down")})

	recorder := performJSON(router, http.MethodDelete, "/sessions/8", "")

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
}

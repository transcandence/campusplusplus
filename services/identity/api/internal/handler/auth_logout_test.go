package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/model"
	"github.com/transcandence/campusplusplus/services/identity/api/internal/service"
)

type logoutSessionRepository struct {
	session           *model.Session
	findErr           error
	revokeErr         error
	revokedID         int64
	sessions          []model.SessionSummary
	listErr           error
	revokeByUserFound bool
	revokeByUserErr   error
	revokedForUserID  int64
}

func (f *logoutSessionRepository) Create(context.Context, *model.Session) error { return nil }

func (f *logoutSessionRepository) CreateWithLimit(context.Context, *model.Session, int) error {
	return nil
}

func (f *logoutSessionRepository) ListActiveByUserID(context.Context, int64) ([]model.SessionSummary, error) {
	return f.sessions, f.listErr
}

func (f *logoutSessionRepository) RevokeByIDForUser(_ context.Context, userID, _ int64) (bool, error) {
	f.revokedForUserID = userID
	return f.revokeByUserFound, f.revokeByUserErr
}

func (f *logoutSessionRepository) FindByTokenHash(context.Context, []byte) (*model.Session, error) {
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.session == nil {
		return nil, model.ErrSessionNotFound
	}
	return f.session, nil
}

func (f *logoutSessionRepository) Rotate(context.Context, int64, *model.Session) error {
	return nil
}

func (f *logoutSessionRepository) RevokeByID(_ context.Context, sessionID int64) error {
	f.revokedID = sessionID
	return f.revokeErr
}

func (f *logoutSessionRepository) RevokeAllByUserID(context.Context, int64) error { return nil }

func (f *logoutSessionRepository) RevokeFamily(context.Context, uuid.UUID) error { return nil }

func newLogoutRouter(repo *logoutSessionRepository) *gin.Engine {
	gin.SetMode(gin.TestMode)
	authService := service.NewAuthService(nil, repo, nil, time.Minute, time.Hour, 10)
	authHandler := NewAuthHandler(authService, nil)

	router := gin.New()
	router.POST("/auth/logout", authHandler.Logout)
	return router
}

func performLogout(router http.Handler, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/auth/logout", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, req)
	return recorder
}

func decodeProblem(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()

	if got := recorder.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("Content-Type = %q, want application/problem+json", got)
	}
	var problem map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &problem); err != nil {
		t.Fatalf("decoding problem body: %v", err)
	}
	return problem
}

func TestLogoutRequiresBody(t *testing.T) {
	router := newLogoutRouter(&logoutSessionRepository{})

	recorder := performLogout(router, "")

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
	problem := decodeProblem(t, recorder)
	if problem["code"] != model.ErrCodeValidationFailed {
		t.Errorf("problem code = %v, want %q", problem["code"], model.ErrCodeValidationFailed)
	}
}

func TestLogoutRequiresRefreshToken(t *testing.T) {
	router := newLogoutRouter(&logoutSessionRepository{})

	recorder := performLogout(router, `{"refreshToken":""}`)

	if recorder.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusUnprocessableEntity)
	}
	problem := decodeProblem(t, recorder)
	if problem["detail"] != "Refresh token is required" {
		t.Errorf("problem detail = %v, want %q", problem["detail"], "Refresh token is required")
	}
}

func TestLogoutUnknownSessionIsIdempotent(t *testing.T) {
	router := newLogoutRouter(&logoutSessionRepository{})

	recorder := performLogout(router, `{"refreshToken":"unknown-token"}`)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store", got)
	}
}

func TestLogoutRevokesSession(t *testing.T) {
	repo := &logoutSessionRepository{session: &model.Session{ID: 77}}
	router := newLogoutRouter(repo)

	recorder := performLogout(router, `{"refreshToken":"valid-token"}`)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if repo.revokedID != 77 {
		t.Errorf("revoked session id = %d, want 77", repo.revokedID)
	}
}

func TestLogoutPropagatesLookupError(t *testing.T) {
	router := newLogoutRouter(&logoutSessionRepository{findErr: errors.New("database unavailable")})

	recorder := performLogout(router, `{"refreshToken":"valid-token"}`)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	problem := decodeProblem(t, recorder)
	if problem["code"] != model.ErrCodeInternalError {
		t.Errorf("problem code = %v, want %q", problem["code"], model.ErrCodeInternalError)
	}
}

func TestLogoutPropagatesRevokeError(t *testing.T) {
	router := newLogoutRouter(&logoutSessionRepository{
		session:   &model.Session{ID: 77},
		revokeErr: errors.New("database unavailable"),
	})

	recorder := performLogout(router, `{"refreshToken":"valid-token"}`)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
	}
	problem := decodeProblem(t, recorder)
	if problem["code"] != model.ErrCodeInternalError {
		t.Errorf("problem code = %v, want %q", problem["code"], model.ErrCodeInternalError)
	}
}

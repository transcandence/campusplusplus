package middleware_test

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/transcandence/campusplusplus/api-gateway/internal/middleware"
)

func generateTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}
	return priv
}

// signRS256JWT creates a standard RS256 JWT matching your claims payload
// without needing extra test dependencies.
func signRS256JWT(t *testing.T, priv *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()

	headerJSON, err := json.Marshal(map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("failed to marshal JWT header: %v", err)
	}

	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("failed to marshal JWT claims: %v", err)
	}

	unsigned := base64.RawURLEncoding.EncodeToString(headerJSON) + "." +
		base64.RawURLEncoding.EncodeToString(claimsJSON)

	digest := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("failed to sign JWT: %v", err)
	}

	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func assertUnauthorizedProblemDetails(t *testing.T, rec *httptest.ResponseRecorder, expectedPath string) {
	t.Helper()

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401 Unauthorized, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("expected Content-Type %q, got %q", "application/problem+json", got)
	}

	var problem middleware.ProblemDetails
	if err := json.NewDecoder(rec.Body).Decode(&problem); err != nil {
		t.Fatalf("failed to decode RFC 9457 Problem Details response: %v", err)
	}

	if problem.Status != http.StatusUnauthorized {
		t.Errorf("expected problem.Status 401, got %d", problem.Status)
	}
	if problem.Title != "Unauthorized" {
		t.Errorf("expected problem.Title %q, got %q", "Unauthorized", problem.Title)
	}
	if problem.Instance != expectedPath {
		t.Errorf("expected problem.Instance %q, got %q", expectedPath, problem.Instance)
	}
	if problem.Detail == "" {
		t.Error("expected non-empty problem.Detail")
	}
}

func TestJwtAuth_PublicRoutesBypassAuthentication(t *testing.T) {
	privKey := generateTestRSAKey(t)

	publicRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/auth/login"},
		{http.MethodPost, "/api/v1/auth/refresh"},
		{http.MethodGet, "/healthz"},
	}

	for _, tc := range publicRoutes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			nextCalled := false
			downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true

				// Spoofed headers must still be stripped on public routes
				for _, h := range []string{"X-User-Id", "X-User-Role", "X-Campus-Id", "X-Session-Id"} {
					if got := r.Header.Get(h); got != "" {
						t.Errorf("expected spoofed header %s to be stripped, got %q", h, got)
					}
				}

				w.WriteHeader(http.StatusOK)
			})

			handler := middleware.JwtAuth(&privKey.PublicKey, downstream)

			req := httptest.NewRequest(tc.method, tc.path, nil)
			req.Header.Set("X-User-Id", "9999")
			req.Header.Set("X-User-Role", "admin")
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if !nextCalled {
				t.Fatal("expected downstream handler to be called for whitelisted route")
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("expected status 200 OK, got %d", rec.Code)
			}
		})
	}
}

func TestJwtAuth_ProtectedRoutesRejectInvalidTokens(t *testing.T) {
	privKey := generateTestRSAKey(t)
	wrongPrivKey := generateTestRSAKey(t)

	expiredToken := signRS256JWT(t, privKey, map[string]any{
		"user_id":    int64(101),
		"role":       "student",
		"campus_id":  int64(1),
		"session_id": int64(55),
		"exp":        time.Now().Add(-10 * time.Minute).Unix(),
	})

	wrongSigToken := signRS256JWT(t, wrongPrivKey, map[string]any{
		"user_id":    int64(101),
		"role":       "student",
		"campus_id":  int64(1),
		"session_id": int64(55),
		"exp":        time.Now().Add(15 * time.Minute).Unix(),
	})

	tests := []struct {
		name       string
		method     string
		path       string
		authHeader string
	}{
		{
			name:       "missing Authorization header",
			method:     http.MethodGet,
			path:       "/api/v1/housing/rooms",
			authHeader: "",
		},
		{
			name:       "non-Bearer authorization scheme",
			method:     http.MethodGet,
			path:       "/api/v1/housing/rooms",
			authHeader: "Basic dXNlcjpwYXNz",
		},
		{
			name:       "Bearer without token",
			method:     http.MethodGet,
			path:       "/api/v1/housing/rooms",
			authHeader: "Bearer ",
		},
		{
			name:       "malformed JWT string",
			method:     http.MethodGet,
			path:       "/api/v1/housing/rooms",
			authHeader: "Bearer invalid.jwt.token",
		},
		{
			name:       "expired JWT",
			method:     http.MethodGet,
			path:       "/api/v1/housing/rooms",
			authHeader: "Bearer " + expiredToken,
		},
		{
			name:       "JWT signed with wrong RSA private key",
			method:     http.MethodGet,
			path:       "/api/v1/housing/rooms",
			authHeader: "Bearer " + wrongSigToken,
		},
		{
			name:       "wrong HTTP method on whitelisted path still requires auth",
			method:     http.MethodGet,
			path:       "/api/v1/auth/login",
			authHeader: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			nextCalled := false
			downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
			})

			handler := middleware.JwtAuth(&privKey.PublicKey, downstream)

			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			if nextCalled {
				t.Fatal("downstream handler must NOT be called when authentication fails")
			}
			assertUnauthorizedProblemDetails(t, rec, tc.path)
		})
	}
}

func TestJwtAuth_ValidTokenPropagatesIdentityHeaders(t *testing.T) {
	privKey := generateTestRSAKey(t)

	validToken := signRS256JWT(t, privKey, map[string]any{
			"user_id":    int64(42),
			"role":       "student",
			"campus_id":  int64(7),
			"sid":        int64(9001),
			"exp":        time.Now().Add(15 * time.Minute).Unix(),
		})

	var receivedHeaders http.Header
	downstream := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.JwtAuth(&privKey.PublicKey, downstream)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/transport/shuttles", nil)
	req.Header.Set("Authorization", "Bearer "+validToken)
	// Simulate client attempting to spoof identity headers
	req.Header.Set("X-User-Id", "1")
	req.Header.Set("X-User-Role", "superadmin")
	req.Header.Set("X-Campus-Id", "99")
	req.Header.Set("X-Session-Id", "111")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	expected := map[string]string{
		"X-User-Id":    "42",
		"X-User-Role":  "student",
		"X-Campus-Id":  "7",
		"X-Session-Id": "9001",
	}

	for header, want := range expected {
		if got := receivedHeaders.Get(header); got != want {
			t.Errorf("header %s = %q, want %q", header, got, want)
		}
	}
}
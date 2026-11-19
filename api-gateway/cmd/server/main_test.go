package main

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/transcandence/campusplusplus/api-gateway/internal/middleware"
	"github.com/transcandence/campusplusplus/api-gateway/internal/proxy"
)

type gatewayTestEnv struct {
	gatewayURL   string
	privKey      *rsa.PrivateKey
	identitySrv  *httptest.Server
	housingSrv   *httptest.Server
	transportSrv *httptest.Server
	gatewaySrv   *httptest.Server
}

type capturedUpstreamReq struct {
	Method string
	Path   string
	Header http.Header
	Body   string
}

func setupGatewayTestEnv(t *testing.T, onIdentity, onHousing, onTransport http.HandlerFunc) *gatewayTestEnv {
	t.Helper()

	privKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test RSA key: %v", err)
	}

	defaultHandler := func(serviceName string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"service": serviceName,
				"path":    r.URL.Path,
			})
		}
	}

	if onIdentity == nil {
		onIdentity = defaultHandler("identity")
	}
	if onHousing == nil {
		onHousing = defaultHandler("housing")
	}
	if onTransport == nil {
		onTransport = defaultHandler("transport")
	}

	identitySrv := httptest.NewServer(onIdentity)
	housingSrv := httptest.NewServer(onHousing)
	transportSrv := httptest.NewServer(onTransport)

	gatewayProxy, err := proxy.NewProxy(identitySrv.URL, housingSrv.URL, transportSrv.URL)
	if err != nil {
		identitySrv.Close()
		housingSrv.Close()
		transportSrv.Close()
		t.Fatalf("failed to construct proxy: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	mux.Handle("/", gatewayProxy)

	protectedHandler := middleware.JwtAuth(&privKey.PublicKey, mux)
	gatewaySrv := httptest.NewServer(protectedHandler)

	t.Cleanup(func() {
		gatewaySrv.Close()
		identitySrv.Close()
		housingSrv.Close()
		transportSrv.Close()
	})

	return &gatewayTestEnv{
		gatewayURL:   gatewaySrv.URL,
		privKey:      privKey,
		identitySrv:  identitySrv,
		housingSrv:   housingSrv,
		transportSrv: transportSrv,
		gatewaySrv:   gatewaySrv,
	}
}

func signTestJWT(t *testing.T, priv *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()

	headerBytes, err := json.Marshal(map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	})
	if err != nil {
		t.Fatalf("marshal header: %v", err)
	}

	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}

	unsigned := base64.RawURLEncoding.EncodeToString(headerBytes) + "." +
		base64.RawURLEncoding.EncodeToString(claimsBytes)

	digest := sha256.Sum256([]byte(unsigned))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	return unsigned + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func assertProblemJSON(t *testing.T, resp *http.Response, expectedStatus int) map[string]any {
	t.Helper()

	if resp.StatusCode != expectedStatus {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected HTTP status %d, got %d (body: %s)", expectedStatus, resp.StatusCode, string(body))
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("expected Content-Type %q, got %q", "application/problem+json", ct)
	}

	var problem map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&problem); err != nil {
		t.Fatalf("failed to decode problem+json body: %v", err)
	}

	if statusVal, ok := problem["status"].(float64); !ok || int(statusVal) != expectedStatus {
		t.Errorf("expected problem.status = %d, got %v", expectedStatus, problem["status"])
	}
	if title, ok := problem["title"].(string); !ok || title == "" {
		t.Errorf("expected non-empty problem.title, got %v", problem["title"])
	}
	if detail, ok := problem["detail"].(string); !ok || detail == "" {
		t.Errorf("expected non-empty problem.detail, got %v", problem["detail"])
	}

	return problem
}

// -----------------------------------------------------------------------------
// Scenario 1: Public Pass-Through
// -----------------------------------------------------------------------------

func TestIntegration_PublicPassThrough_Login(t *testing.T) {
	var captured capturedUpstreamReq

	env := setupGatewayTestEnv(t, func(w http.ResponseWriter, r *http.Request) {
		rawBody, _ := io.ReadAll(r.Body)
		captured = capturedUpstreamReq{
			Method: r.Method,
			Path:   r.URL.Path,
			Header: r.Header.Clone(),
			Body:   string(rawBody),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"access_token":"jwt-abc","token_type":"Bearer"}`))
	}, nil, nil)

	payload := `{"email":"student@um6p.ma","password":"secret"}`
	req, err := http.NewRequest(http.MethodPost, env.gatewayURL+"/api/v1/auth/login", bytes.NewBufferString(payload))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	// Spoofed identity headers must be stripped even on public routes
	req.Header.Set("X-User-Id", "999")
	req.Header.Set("X-User-Role", "admin")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"access_token":"jwt-abc","token_type":"Bearer"}` {
		t.Errorf("unexpected response body: %s", string(body))
	}

	if captured.Method != http.MethodPost {
		t.Errorf("expected upstream POST, got %s", captured.Method)
	}
	if captured.Path != "/auth/login" {
		t.Errorf("expected upstream path /auth/login, got %s", captured.Path)
	}
	if captured.Body != payload {
		t.Errorf("expected upstream body %s, got %s", payload, captured.Body)
	}
	if got := captured.Header.Get("X-User-Id"); got != "" {
		t.Errorf("expected spoofed X-User-Id to be stripped, got %q", got)
	}
	if got := captured.Header.Get("X-User-Role"); got != "" {
		t.Errorf("expected spoofed X-User-Role to be stripped, got %q", got)
	}
}

// -----------------------------------------------------------------------------
// Scenario 2: Protected Route with Valid Token
// -----------------------------------------------------------------------------

func TestIntegration_ProtectedRoute_ValidTokenInjectsHeadersAndProxies(t *testing.T) {
	var captured capturedUpstreamReq

	env := setupGatewayTestEnv(t, nil, func(w http.ResponseWriter, r *http.Request) {
		captured = capturedUpstreamReq{
			Method: r.Method,
			Path:   r.URL.Path,
			Header: r.Header.Clone(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"rooms":[{"id":101,"building":"Atlas"}]}`))
	}, nil)

	token := signTestJWT(t, env.privKey, map[string]any{
			"user_id":   int64(42),
			"role":      "student",
			"campus_id": int64(3),
			"sid":       int64(8801),
			"exp":       time.Now().Add(15 * time.Minute).Unix(),
		})

	req, err := http.NewRequest(http.MethodGet, env.gatewayURL+"/api/v1/housing/rooms", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	// Attempt header spoofing from client
	req.Header.Set("X-User-Id", "1")
	req.Header.Set("X-User-Role", "superadmin")
	req.Header.Set("X-Campus-Id", "99")
	req.Header.Set("X-Session-Id", "1")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	if captured.Path != "/housing/rooms" {
		t.Errorf("expected stripped upstream path /housing/rooms, got %s", captured.Path)
	}

	expectedHeaders := map[string]string{
		"X-User-Id":    "42",
		"X-User-Role":  "student",
		"X-Campus-Id":  "3",
		"X-Session-Id": "8801",
	}
	for k, want := range expectedHeaders {
		if got := captured.Header.Get(k); got != want {
			t.Errorf("upstream header %s = %q, want %q", k, got, want)
		}
	}
	if captured.Header.Get("X-Request-Id") == "" {
		t.Error("expected X-Request-Id to be injected by gateway director")
	}
}

// -----------------------------------------------------------------------------
// Scenario 3: Token Rejections
// -----------------------------------------------------------------------------

func TestIntegration_TokenRejections_Return401ProblemJSON(t *testing.T) {
	upstreamHit := false
	env := setupGatewayTestEnv(t, nil, func(w http.ResponseWriter, r *http.Request) {
		upstreamHit = true
		w.WriteHeader(http.StatusOK)
	}, nil)

	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate secondary RSA key: %v", err)
	}

	expiredToken := signTestJWT(t, env.privKey, map[string]any{
		"user_id":    int64(42),
		"role":       "student",
		"campus_id":  int64(3),
		"session_id": int64(8801),
		"exp":        time.Now().Add(-5 * time.Minute).Unix(),
	})

	tamperedSigToken := signTestJWT(t, otherKey, map[string]any{
		"user_id":    int64(42),
		"role":       "student",
		"campus_id":  int64(3),
		"session_id": int64(8801),
		"exp":        time.Now().Add(15 * time.Minute).Unix(),
	})

	cases := []struct {
		name       string
		authHeader string
	}{
		{
			name:       "missing token",
			authHeader: "",
		},
		{
			name:       "expired token",
			authHeader: "Bearer " + expiredToken,
		},
		{
			name:       "tampered signature",
			authHeader: "Bearer " + tamperedSigToken,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			upstreamHit = false

			req, err := http.NewRequest(http.MethodGet, env.gatewayURL+"/api/v1/housing/rooms", nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			if tc.authHeader != "" {
				req.Header.Set("Authorization", tc.authHeader)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			assertProblemJSON(t, resp, http.StatusUnauthorized)

			if upstreamHit {
				t.Fatal("upstream service must not be contacted on 401 rejection")
			}
		})
	}
}

// -----------------------------------------------------------------------------
// Scenario 4: Routing Errors (404 Not Found & 502 Bad Gateway)
// -----------------------------------------------------------------------------

func TestIntegration_RoutingErrors(t *testing.T) {
	t.Run("non-existent route returns 404 Not Found Problem JSON", func(t *testing.T) {
		env := setupGatewayTestEnv(t, nil, nil, nil)

		validToken := signTestJWT(t, env.privKey, map[string]any{
			"user_id":    int64(42),
			"role":       "student",
			"campus_id":  int64(3),
			"session_id": int64(8801),
			"exp":        time.Now().Add(15 * time.Minute).Unix(),
		})

		req, err := http.NewRequest(http.MethodGet, env.gatewayURL+"/api/v1/nonexistent/resource", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+validToken)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		assertProblemJSON(t, resp, http.StatusNotFound)
	})

	t.Run("downstream service unavailable returns 502 Bad Gateway Problem JSON with X-Request-Id", func(t *testing.T) {
		env := setupGatewayTestEnv(t, nil, nil, nil)

		// Shut down the housing upstream server to simulate connection refused
		env.housingSrv.Close()

		validToken := signTestJWT(t, env.privKey, map[string]any{
			"user_id":    int64(42),
			"role":       "student",
			"campus_id":  int64(3),
			"session_id": int64(8801),
			"exp":        time.Now().Add(15 * time.Minute).Unix(),
		})

		req, err := http.NewRequest(http.MethodGet, env.gatewayURL+"/api/v1/housing/rooms", nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+validToken)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("request failed: %v", err)
		}
		defer resp.Body.Close()

		problem := assertProblemJSON(t, resp, http.StatusBadGateway)

		headerReqID := resp.Header.Get("X-Request-Id")
		if headerReqID == "" {
			t.Error("expected non-empty X-Request-Id response header on 502 Bad Gateway")
		}

		bodyReqID, _ := problem["requestId"].(string)
		if bodyReqID == "" {
			t.Error("expected non-empty requestId field in Problem JSON body")
		}
		if headerReqID != bodyReqID {
			t.Errorf("X-Request-Id header (%q) does not match body requestId (%q)", headerReqID, bodyReqID)
		}
	})
}
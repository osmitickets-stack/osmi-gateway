package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAuthenticatedTokenBoundary(t *testing.T) {
	const user = "11111111-1111-4111-8111-111111111111"
	const secret = "0123456789abcdef0123456789abcdef"
	cases := []struct {
		name   string
		claims jwt.MapClaims
		method jwt.SigningMethod
		key    string
		allow  bool
	}{
		{"valid", jwt.MapClaims{"user_id": user, "exp": time.Now().Add(time.Minute).Unix()}, jwt.SigningMethodHS256, secret, true},
		{"expired", jwt.MapClaims{"user_id": user, "exp": time.Now().Add(-time.Minute).Unix()}, jwt.SigningMethodHS256, secret, false},
		{"no expiration", jwt.MapClaims{"user_id": user}, jwt.SigningMethodHS256, secret, false},
		{"wrong signature", jwt.MapClaims{"user_id": user, "exp": time.Now().Add(time.Minute).Unix()}, jwt.SigningMethodHS256, "wrong-key", false},
		{"wrong algorithm", jwt.MapClaims{"user_id": user, "exp": time.Now().Add(time.Minute).Unix()}, jwt.SigningMethodHS512, secret, false},
		{"numeric user", jwt.MapClaims{"user_id": 42, "exp": time.Now().Add(time.Minute).Unix()}, jwt.SigningMethodHS256, secret, false},
		{"malformed user", jwt.MapClaims{"user_id": "attacker", "exp": time.Now().Add(time.Minute).Unix()}, jwt.SigningMethodHS256, secret, false},
		{"nil UUID", jwt.MapClaims{"user_id": "00000000-0000-0000-0000-000000000000", "exp": time.Now().Add(time.Minute).Unix()}, jwt.SigningMethodHS256, secret, false},
		{"future issue time", jwt.MapClaims{"user_id": user, "exp": time.Now().Add(time.Hour).Unix(), "iat": time.Now().Add(time.Minute).Unix()}, jwt.SigningMethodHS256, secret, false},
		{"not active yet", jwt.MapClaims{"user_id": user, "exp": time.Now().Add(time.Hour).Unix(), "nbf": time.Now().Add(time.Minute).Unix()}, jwt.SigningMethodHS256, secret, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			token, err := jwt.NewWithClaims(tc.method, tc.claims).SignedString([]byte(tc.key))
			if err != nil {
				t.Fatal(err)
			}
			called := false
			handler := Auth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				got, ok := GetUserIDFromContext(r.Context())
				if !ok || got != user {
					t.Fatalf("wrong identity: %q", got)
				}
				forwarded, ok := ValidatedAccessToken(r.Context())
				if !ok || forwarded != token {
					t.Fatal("validated token missing")
				}
				w.WriteHeader(http.StatusNoContent)
			}), secret, nil)
			req := httptest.NewRequest(http.MethodPost, "/v1/tickets/validate", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Grpc-Metadata-User-Id", "22222222-2222-4222-8222-222222222222")
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, req)
			if tc.allow {
				if out.Code != 204 || !called {
					t.Fatalf("valid request denied: %d", out.Code)
				}
			} else if out.Code != 401 || called {
				t.Fatalf("invalid token passed: %d", out.Code)
			}
		})
	}
	if _, err := AuthenticatedUserID("anything", ""); err == nil {
		t.Fatal("empty secret accepted")
	}
}

func TestAuthRejectsDuplicateAuthorizationAndPathPrefix(t *testing.T) {
	handler := AuthExcludingPaths(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusNoContent) }), []string{"/v1/auth/login"}, "secret", nil)
	for _, path := range []string{"/v1/auth/loginevil", "/v1/tickets/validate", "/v1/tickets/check-in"} {
		req := httptest.NewRequest("POST", path, strings.NewReader("{}"))
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, req)
		if out.Code != 401 {
			t.Fatalf("authentication bypass at %s: %d", path, out.Code)
		}
	}
	req := httptest.NewRequest("POST", "/v1/tickets/validate", nil)
	req.Header.Add("Authorization", "Bearer a")
	req.Header.Add("Authorization", "Bearer b")
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, req)
	if out.Code != 401 {
		t.Fatal(out.Code)
	}
	out = httptest.NewRecorder()
	handler.ServeHTTP(out, httptest.NewRequest("POST", "/v1/auth/login", nil))
	if out.Code != 204 {
		t.Fatal("public login blocked")
	}
}

func TestPublicUserRegistrationDoesNotExposeOtherUserRoutes(t *testing.T) {
	handler := AuthExcludingPaths(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}), []string{"/v1/users"}, "test-only-secret", nil)
	for _, tc := range []struct {
		method, path string
		want         int
	}{
		{http.MethodPost, "/v1/users", http.StatusNoContent},
		{http.MethodGet, "/v1/users", http.StatusUnauthorized},
		{http.MethodPatch, "/v1/users/11111111-1111-4111-8111-111111111111", http.StatusUnauthorized},
		{http.MethodDelete, "/v1/users/11111111-1111-4111-8111-111111111111", http.StatusUnauthorized},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
			if response.Code != tc.want {
				t.Fatalf("status=%d want=%d", response.Code, tc.want)
			}
		})
	}
}

func TestRevocationFailsClosed(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": "11111111-1111-4111-8111-111111111111", "exp": time.Now().Add(time.Minute).Unix()}).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		revoked bool
		err     error
		code    int
	}{{"revoked", true, nil, 401}, {"store unavailable", false, errors.New("private store detail"), 503}, {"not revoked", false, nil, 204}} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			checks := 0
			handler := authWithRevocationCheck(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true; w.WriteHeader(204) }), secret, func(ctx context.Context, got string) (bool, error) {
				checks++
				if got != token {
					t.Fatal("wrong token checked")
				}
				return tc.revoked, tc.err
			})
			req := httptest.NewRequest("POST", "/v1/tickets/validate", nil)
			req.Header.Set("Authorization", "Bearer "+token)
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, req)
			if out.Code != tc.code || called != (tc.code == 204) || checks != 1 {
				t.Fatalf("status=%d handler=%v checks=%d", out.Code, called, checks)
			}
			if strings.Contains(out.Body.String(), "private store detail") {
				t.Fatal("internal error leaked")
			}
			req.Header.Set("Authorization", "Bearer invalid")
			out = httptest.NewRecorder()
			handler.ServeHTTP(out, req)
			if out.Code != 401 || checks != 1 {
				t.Fatal("invalid token reached revocation store")
			}
		})
	}
}

func TestOptionalAuthenticationOnPublicResources(t *testing.T) {
	const user = "11111111-1111-4111-8111-111111111111"
	const secret = "0123456789abcdef0123456789abcdef"
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"user_id": user, "exp": time.Now().Add(time.Minute).Unix()}).SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path, token string
		wantUser          bool
		code              int
	}{{"public guest", "/v1/events", "", false, 204}, {"public authenticated", "/v1/events", token, true, 204}, {"profile under public users prefix", "/v1/users/me", token, true, 204}, {"bad resource token", "/v1/events", "invalid", false, 401}, {"login ignores stale access header", "/v1/auth/login", "invalid", false, 204}, {"refresh uses its own body credential", "/v1/auth/refresh", "invalid", false, 204}} {
		t.Run(tc.name, func(t *testing.T) {
			handler := AuthExcludingPaths(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				id, ok := GetUserIDFromContext(r.Context())
				if tc.wantUser {
					if !ok || id != user {
						t.Fatal("optional identity lost")
					}
				} else if ok {
					t.Fatal("unexpected identity")
				}
				w.WriteHeader(204)
			}), []string{"/v1/events", "/v1/users", "/v1/auth/login", "/v1/auth/refresh"}, secret, nil)
			req := httptest.NewRequest("GET", tc.path, nil)
			if tc.token != "" {
				req.Header.Set("Authorization", "Bearer "+tc.token)
			}
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, req)
			if out.Code != tc.code {
				t.Fatalf("status=%d", out.Code)
			}
		})
	}
}

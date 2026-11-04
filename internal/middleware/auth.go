package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/osmitickets-stack/osmi-gateway/internal/cache"
)

type contextKey string

const (
	UserIDKey      contextKey = "user_id"
	RequestIDKey   contextKey = "requestID"
	accessTokenKey contextKey = "validated_access_token"
)

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		ctx := context.WithValue(r.Context(), RequestIDKey, requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetRequestID(ctx context.Context) string {
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

// Auth valida el token JWT y verifica que no esté en blacklist
func Auth(next http.Handler, jwtSecret string, redisClient *cache.RedisClient) http.Handler {
	var check func(context.Context, string) (bool, error)
	if redisClient != nil {
		check = redisClient.IsBlacklisted
	}
	return authWithRevocationCheck(next, jwtSecret, check)
}

func authWithRevocationCheck(next http.Handler, jwtSecret string, check func(context.Context, string) (bool, error)) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeaders := r.Header.Values("Authorization")
		if len(authHeaders) != 1 {
			http.Error(w, "authorization header required", http.StatusUnauthorized)
			return
		}
		authHeader := authHeaders[0]
		if authHeader == "" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"missing authorization header"}`))
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid token format"}`))
			return
		}

		tokenString := parts[1]

		userID, err := AuthenticatedUserID(tokenString, jwtSecret)
		if err != nil {
			http.Error(w, "invalid or expired token", http.StatusUnauthorized)
			return
		}

		// Consultar revocación únicamente para tokens auténticos.
		if check != nil {
			blacklisted, err := check(r.Context(), tokenString)
			if err != nil {
				log.Printf("Token revocation check unavailable")
				http.Error(w, "authentication temporarily unavailable", http.StatusServiceUnavailable)
				return
			}
			if blacklisted {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"token has been revoked"}`))
				return
			}
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		ctx = context.WithValue(ctx, accessTokenKey, tokenString)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthExcludingPaths aplica autenticación excepto en rutas específicas
func AuthExcludingPaths(next http.Handler, excludePaths []string, jwtSecret string, redisClient *cache.RedisClient) http.Handler {
	authenticated := Auth(next, jwtSecret, redisClient)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, path := range excludePaths {
			if r.URL.Path == strings.TrimSuffix(path, "/") || strings.HasPrefix(r.URL.Path, strings.TrimSuffix(path, "/")+"/") {
				if path == "/v1/users" && (r.URL.Path != path || r.Method != http.MethodPost) {
					authenticated.ServeHTTP(w, r)
					return
				}
				// Resource routes may be public and still carry a legitimate user.
				dedicatedAuth := r.URL.Path == "/v1/auth/login" || r.URL.Path == "/v1/auth/refresh" || r.URL.Path == "/health" || r.URL.Path == "/v1/webhooks/stripe"
				if len(r.Header.Values("Authorization")) > 0 && !dedicatedAuth {
					authenticated.ServeHTTP(w, r)
				} else {
					next.ServeHTTP(w, r)
				}
				return
			}
		}
		authenticated.ServeHTTP(w, r)
	})
}

func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}

// ValidatedAccessToken is populated only after signature and revocation checks.
func ValidatedAccessToken(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(accessTokenKey).(string)
	return token, ok
}

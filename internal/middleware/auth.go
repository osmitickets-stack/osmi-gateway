package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"

	"github.com/franciscozamorau/osmi-gateway/internal/cache"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type contextKey string

const (
	UserIDKey    contextKey = "user_id"
	RequestIDKey contextKey = "requestID"
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
	secretBytes := []byte(jwtSecret)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
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

		// 🔥 Verificar si el token está en blacklist
		if redisClient != nil {
			blacklisted, err := redisClient.IsBlacklisted(r.Context(), tokenString)
			if err != nil {
				log.Printf("⚠️ Error checking blacklist: %v", err)
			}
			if blacklisted {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":"token has been revoked"}`))
				return
			}
		}

		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return secretBytes, nil
		})

		if err != nil || !token.Valid {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid or expired token"}`))
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"invalid token claims"}`))
			return
		}

		var userID string
		if uid, ok := claims["user_id"].(string); ok {
			userID = uid
		} else if uid, ok := claims["user_id"].(float64); ok {
			userID = string(rune(int64(uid)))
		}

		if userID == "" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"user_id not found in token"}`))
			return
		}

		ctx := context.WithValue(r.Context(), UserIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AuthExcludingPaths aplica autenticación excepto en rutas específicas
func AuthExcludingPaths(next http.Handler, excludePaths []string, jwtSecret string, redisClient *cache.RedisClient) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, path := range excludePaths {
			if r.URL.Path == path || strings.HasPrefix(r.URL.Path, path) {
				next.ServeHTTP(w, r)
				return
			}
		}
		Auth(next, jwtSecret, redisClient).ServeHTTP(w, r)
	})
}

func GetUserIDFromContext(ctx context.Context) (string, bool) {
	userID, ok := ctx.Value(UserIDKey).(string)
	return userID, ok
}

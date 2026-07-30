// internal/middleware/cors.go
package middleware

import (
	"net/http"

	"github.com/osmitickets-stack/osmi-gateway/internal/config"
)

// CORS maneja Cross-Origin Resource Sharing
func CORS(next http.Handler) http.Handler {
	cfg := config.Load()

	// Crear un mapa para búsqueda rápida de orígenes permitidos
	allowedOrigins := make(map[string]bool)
	for _, origin := range cfg.CORSOrigins {
		allowedOrigins[origin] = true
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Verificar si el origen está permitido
		var allowOrigin string
		if allowedOrigins[origin] {
			allowOrigin = origin
		} else if len(cfg.CORSOrigins) == 1 && cfg.CORSOrigins[0] == "*" {
			// Solo para desarrollo
			allowOrigin = "*"
		} else {
			// Si no está permitido, no devolver header CORS
			// Esto bloqueará la petición
		}

		// Configurar headers CORS solo si el origen está permitido
		if allowOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization, X-Request-ID")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		}

		// Manejar preflight requests
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

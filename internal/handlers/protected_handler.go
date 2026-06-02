// internal/handlers/protected_handler.go
package handlers

import (
	"net/http"
)

func ProtectedHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message":"Acceso autorizado a ruta protegida"}`))
	}
}

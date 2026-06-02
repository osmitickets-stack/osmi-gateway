package auth

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/franciscozamorau/osmi-protobuf/gen/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at,omitempty"`
	User      struct {
		UserID    string `json:"user_id"`
		Email     string `json:"email"`
		Name      string `json:"name"`
		Role      string `json:"role"`
		CreatedAt string `json:"created_at"`
	} `json:"user"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type AuthHandler struct {
	grpcClient pb.OsmiServiceClient
}

func NewAuthHandler(grpcConn *grpc.ClientConn) *AuthHandler {
	return &AuthHandler{
		grpcClient: pb.NewOsmiServiceClient(grpcConn),
	}
}

// Login maneja POST /v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		sendJSONError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		sendJSONError(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	// Llamar al servidor gRPC
	grpcReq := &pb.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Añadir metadata si es necesario (ej. tracing)
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs())

	resp, err := h.grpcClient.Login(ctx, grpcReq)
	if err != nil {
		log.Printf("❌ Error en login gRPC: %v", err)
		sendJSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Construir respuesta
	response := LoginResponse{
		Token:     resp.Token,
		ExpiresAt: resp.ExpiresAt.AsTime().Format(time.RFC3339),
	}
	response.User.UserID = resp.User.UserId
	response.User.Email = resp.User.Email
	response.User.Name = resp.User.Name
	response.User.Role = resp.User.Role
	response.User.CreatedAt = resp.User.CreatedAt.AsTime().Format(time.RFC3339)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

func sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

package grpc

import (
	"context"

	"github.com/google/uuid"
	"github.com/osmitickets-stack/osmi-gateway/internal/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

type AuthInterceptor struct {
	jwtSecret []byte
}

func NewAuthInterceptor(jwtSecret string) *AuthInterceptor {
	return &AuthInterceptor{
		jwtSecret: []byte(jwtSecret),
	}
}

// UnaryClientInterceptor inyecta user_id del JWT como metadata gRPC
func (a *AuthInterceptor) UnaryClientInterceptor(
	ctx context.Context,
	method string,
	req, reply interface{},
	cc *grpc.ClientConn,
	invoker grpc.UnaryInvoker,
	opts ...grpc.CallOption,
) error {
	// Extraer user_id del contexto HTTP (inyectado por el middleware Auth)
	// Preservar la metadata no relacionada con identidad.
	// Nunca confiar en un user-id que ya venga en el contexto.
	outgoingMD, _ := metadata.FromOutgoingContext(ctx)
	outgoingMD = outgoingMD.Copy()

	for _, key := range []string{"user-id", "user_id", "x-user-id", "grpc-metadata-user-id", "role", "roles", "is-superuser", "is-staff", "authorization"} {
		outgoingMD.Delete(key)
	}

	// La única identidad permitida procede del middleware HTTP
	// que verificó el token JWT.
	if userID, ok := middleware.GetUserIDFromContext(ctx); ok && userID != "" {
		id, err := uuid.Parse(userID)
		if err != nil || id == uuid.Nil || id.String() != userID {
			return status.Error(codes.Unauthenticated, "invalid authenticated user identity")
		}
		outgoingMD.Set("user-id", userID)
		if token, ok := middleware.ValidatedAccessToken(ctx); ok && token != "" {
			outgoingMD.Set("authorization", "Bearer "+token)
		}
	}

	ctx = metadata.NewOutgoingContext(ctx, outgoingMD)

	return invoker(ctx, method, req, reply, cc, opts...)
}

// Extrae user_id del token JWT (para cuando no hay contexto HTTP)
func (a *AuthInterceptor) ExtractUserID(tokenString string) (string, error) {
	return middleware.AuthenticatedUserID(tokenString, string(a.jwtSecret))
}

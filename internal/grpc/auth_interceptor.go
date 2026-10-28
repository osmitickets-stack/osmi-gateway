package grpc

import (
	"context"

	"github.com/golang-jwt/jwt/v5"
	"github.com/osmitickets-stack/osmi-gateway/internal/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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

	outgoingMD.Delete("user-id")

	// La única identidad permitida procede del middleware HTTP
	// que verificó el token JWT.
	if userID, ok := middleware.GetUserIDFromContext(ctx); ok && userID != "" {
		outgoingMD.Set("user-id", userID)
	}

	ctx = metadata.NewOutgoingContext(ctx, outgoingMD)

	return invoker(ctx, method, req, reply, cc, opts...)
}

// Extrae user_id del token JWT (para cuando no hay contexto HTTP)
func (a *AuthInterceptor) ExtractUserID(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return a.jwtSecret, nil
	})
	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", jwt.ErrSignatureInvalid
	}

	userID, _ := claims["user_id"].(string)
	return userID, nil
}

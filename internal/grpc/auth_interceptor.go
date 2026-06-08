package grpc

import (
	"context"

	"github.com/golang-jwt/jwt/v5"
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
	if userID, ok := ctx.Value("user_id").(string); ok && userID != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "user-id", userID)
	}

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

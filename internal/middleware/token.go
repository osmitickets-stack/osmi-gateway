package middleware

import (
	"errors"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AuthenticatedUserID accepts only the token format issued by OSMI.
func AuthenticatedUserID(tokenString, jwtSecret string) (string, error) {
	if jwtSecret == "" {
		return "", errors.New("JWT secret is required")
	}
	claims := &struct {
		UserID string `json:"user_id"`
		jwt.RegisteredClaims
	}{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(*jwt.Token) (interface{}, error) { return []byte(jwtSecret), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !token.Valid {
		return "", errors.New("invalid or expired token")
	}
	id, err := uuid.Parse(claims.UserID)
	if err != nil || id == uuid.Nil || id.String() != claims.UserID {
		return "", errors.New("invalid token user identity")
	}
	return claims.UserID, nil
}

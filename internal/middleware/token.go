package middleware

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AuthenticatedUserID accepts only the token format issued by OSMI.
func AuthenticatedUserID(tokenString, jwtSecret string) (string, error) {
	if jwtSecret == "" {
		return "", errors.New("JWT secret is required")
	}
	claims := &struct {
		UserID    string `json:"user_id"`
		TokenType string `json:"token_type"`
		jwt.RegisteredClaims
	}{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(*jwt.Token) (interface{}, error) { return []byte(jwtSecret), nil }, jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
	if err != nil || !token.Valid {
		return "", errors.New("invalid or expired token")
	}
	// Untyped tokens were issued by Login before token_type existed. Keep them
	// valid until expiry; newly issued refresh tokens cannot authorize requests.
	if claims.TokenType != "" && claims.TokenType != "access" {
		return "", errors.New("token is not an access token")
	}
	// Historical Login tokens were untyped and lasted 24 hours. The old
	// refresh-token generator used seven days, so do not accept those as bearer.
	if claims.TokenType == "" && claims.IssuedAt != nil &&
		claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time) > 24*time.Hour+time.Minute {
		return "", errors.New("legacy token lifetime exceeds access duration")
	}
	id, err := uuid.Parse(claims.UserID)
	if err != nil || id == uuid.Nil || id.String() != claims.UserID {
		return "", errors.New("invalid token user identity")
	}
	return claims.UserID, nil
}

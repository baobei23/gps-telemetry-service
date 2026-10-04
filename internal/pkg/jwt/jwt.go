package jwt

import (
	"time"

	"errors"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type TokenManager struct {
	SecretKey     string        `json:"secretKey"`
	AccessExpiry  time.Duration `json:"accessExpiry"`
	RefreshExpiry time.Duration `json:"refreshExpiry"`
}

type Claims struct {
	UserID    string `json:"userID"`
	Email     string `json:"email"`
	TokenType string `json:"tokenType"` // "access" or "refresh"
	jwt.RegisteredClaims
}

func NewManager(secret string, accessMinutes, refreshHours int) *TokenManager {
	return &TokenManager{
		SecretKey:     secret,
		AccessExpiry:  time.Duration(accessMinutes) * time.Minute,
		RefreshExpiry: time.Duration(refreshHours) * time.Hour,
	}
}

// GeneratePair generates both access and refresh tokens
func (tm *TokenManager) GeneratePair(userID, email string) (accessToken, refreshToken, jti string, err error) {
	accessToken, err = tm.generate(userID, email, TokenTypeAccess, tm.AccessExpiry, "")
	if err != nil {
		return "", "", "", err
	}

	jti = uuid.NewString()
	refreshToken, err = tm.generate(userID, email, TokenTypeRefresh, tm.RefreshExpiry, jti)
	if err != nil {
		return "", "", "", err
	}

	return accessToken, refreshToken, jti, nil
}

func (tm *TokenManager) generate(userID, email, tokenType string, expiry time.Duration, jti string) (string, error) {
	claims := Claims{
		UserID:    userID,
		Email:     email,
		TokenType: tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiry)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ID:        jti,
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(tm.SecretKey))
}

// Validate validates the token and returns the claims.
// You can optionally pass expectedType ("access" or "refresh") to enforce token type.
func (tm *TokenManager) Validate(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
		return []byte(tm.SecretKey), nil
	})
	if err != nil {
		return nil, errors.New("unauthorized: invalid token")
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("unauthorized: invalid claims")
}

// GetAccessExpiry returns the duration for access token expiration
func (tm *TokenManager) GetAccessExpiry() time.Duration {
	return tm.AccessExpiry
}

// GetRefreshExpiry returns the duration for refresh token expiration
func (tm *TokenManager) GetRefreshExpiry() time.Duration {
	return tm.RefreshExpiry
}

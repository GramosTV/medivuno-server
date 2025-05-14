package utils

import (
	"fmt"
	"healthcare-app-server/internal/config"
	"healthcare-app-server/internal/models"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims.
type Claims struct {
	UserID string      `json:"user_id"`
	Role   models.Role `json:"role"`
	jwt.RegisteredClaims
}

// GenerateTokens generates both access and refresh tokens for a user.
func GenerateTokens(user *models.User, cfg *config.Config) (accessToken string, refreshToken string, err error) {
	// Generate Access Token
	accessToken, err = generateAccessToken(user, cfg)
	if err != nil {
		return "", "", err
	}

	// Generate Refresh Token
	refreshToken, err = generateRefreshToken(user, cfg)
	if err != nil {
		return "", "", err
	}

	return accessToken, refreshToken, nil
}

func generateAccessToken(user *models.User, cfg *config.Config) (string, error) {
	expirationTime := time.Now().Add(time.Duration(cfg.JWTExpirationMinutes) * time.Minute)
	claims := &Claims{
		UserID: user.ID, // Removed .String() as ID is already a string
		Role:   user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID, // Removed .String() as ID is already a string
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign access token: %w", err)
	}
	return tokenString, nil
}

func generateRefreshToken(user *models.User, cfg *config.Config) (string, error) {
	expirationTime := time.Now().Add(time.Duration(cfg.JWTRefreshExpirationHours) * time.Hour)
	claims := &Claims{
		UserID: user.ID,   // Removed .String() as ID is already a string
		Role:   user.Role, // Include role for potential future use, though typically refresh tokens are simpler
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   user.ID, // Removed .String() as ID is already a string
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(cfg.JWTRefreshSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign refresh token: %w", err)
	}
	return tokenString, nil
}

// ValidateToken validates a JWT token.
func ValidateToken(tokenString string, secretKey string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	return claims, nil
}

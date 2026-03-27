package ratelimiter

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("invalid token")

type ClientClaims struct {
	ClientID string `json:"clientID"`
	jwt.RegisteredClaims
}

func GenerateJWT(clientID, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := ClientClaims{
		ClientID: clientID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   clientID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ValidateJWT(tokenString, secret string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &ClientClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return []byte(secret), nil
	})
	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(*ClientClaims)
	if !ok || !token.Valid {
		return "", ErrInvalidToken
	}

	if claims.ClientID == "" {
		return "", ErrInvalidToken
	}

	return claims.ClientID, nil
}

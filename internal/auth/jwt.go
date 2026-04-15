package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// IssueAccessToken creates a signed HS256 access token with user and session identifiers.
func IssueAccessToken(signingKey []byte, userID, sessionID string, ttl time.Duration) (string, error) {
	if len(signingKey) == 0 {
		return "", errors.New("signing key is required")
	}
	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"sub": userID,
		"sid": sessionID,
		"exp": now.Add(ttl).Unix(),
		"iat": now.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(signingKey)
}

// ParseAccessToken validates an access token and returns user and session identifiers.
func ParseAccessToken(signingKey []byte, tokenString string) (userID, sessionID string, err error) {
	if len(signingKey) == 0 {
		return "", "", errors.New("signing key is required")
	}
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return signingKey, nil
	})
	if err != nil {
		return "", "", err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", "", errors.New("invalid token")
	}
	sub, _ := claims["sub"].(string)
	sid, _ := claims["sid"].(string)
	if sub == "" || sid == "" {
		return "", "", errors.New("missing token claims")
	}
	return sub, sid, nil
}

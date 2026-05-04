package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

func bearerToken(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("authorization header is required")
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || strings.TrimSpace(parts[1]) == "" {
		return "", errors.New("authorization header must be bearer token")
	}

	return strings.TrimSpace(parts[1]), nil
}

func userIDFromJWT(r *http.Request) (string, error) {
	tokenString, err := bearerToken(r)
	if err != nil {
		return "", err
	}

	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	})
	if err != nil || !token.Valid || claims.Subject == "" {
		return "", errors.New("invalid token")
	}

	return claims.Subject, nil
}

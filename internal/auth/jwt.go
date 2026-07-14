package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

type JWTValidator struct {
	publicKey *rsa.PublicKey
	issuer    string
	audience  string
	algorithm string
}

func NewJWTValidator(publicKeyPath, issuer, audience, algorithm string) (*JWTValidator, error) {
	keyBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("не могу прочитать ключ: %w", err)
	}

	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return nil, fmt.Errorf("не правильный pem формат")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("не могу распарсить ключ: %w", err)
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("ключ не rsa")
	}

	return &JWTValidator{
		publicKey: rsaPub,
		issuer:    issuer,
		audience:  audience,
		algorithm: algorithm,
	}, nil
}

func (v *JWTValidator) Validate(req *http.Request) (*jwt.MapClaims, error) {
	authHeader := req.Header.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("нет заголовка авторизации")
	}

	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return nil, fmt.Errorf("не правильный формат")
	}

	tokenString := parts[1]

	parser := jwt.NewParser(
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithValidMethods([]string{v.algorithm}),
	)

	token, err := parser.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return v.publicKey, nil
	})

	if err != nil {
		return nil, fmt.Errorf("токен не прошел проверку: %w", err)
	}

	if !token.Valid {
		return nil, fmt.Errorf("токен не валидный")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("не правильный формат claims")
	}

	return &claims, nil
}

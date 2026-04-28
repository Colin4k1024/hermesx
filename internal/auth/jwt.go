package auth

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTExtractor validates RS256 JWT tokens and extracts identity claims.
type JWTExtractor struct {
	publicKey *rsa.PublicKey
	issuer    string
}

func NewJWTExtractor(publicKey *rsa.PublicKey, issuer string) *JWTExtractor {
	return &JWTExtractor{publicKey: publicKey, issuer: issuer}
}

func (j *JWTExtractor) Extract(r *http.Request) (*AuthContext, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return nil, nil
	}

	tokenStr := strings.TrimPrefix(auth, "Bearer ")

	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return j.publicKey, nil
	}, jwt.WithIssuer(j.issuer), jwt.WithExpirationRequired())

	if err != nil || !token.Valid {
		return nil, nil // not a valid JWT = not this extractor's match
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, nil
	}

	sub, _ := claims.GetSubject()
	tenantID, _ := claims["tenant_id"].(string)
	rolesRaw, _ := claims["roles"].([]any)

	var roles []string
	for _, r := range rolesRaw {
		if s, ok := r.(string); ok {
			roles = append(roles, s)
		}
	}
	if len(roles) == 0 {
		roles = []string{"user"}
	}

	_ = time.Now() // placeholder for future token-age checks

	return &AuthContext{
		Identity:   sub,
		TenantID:   tenantID,
		Roles:      roles,
		AuthMethod: "jwt",
	}, nil
}

package hmac

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"pushotp/issuer"
)

type Issuer struct {
	secret []byte
	ttl    time.Duration
}

func New(secret string, ttl time.Duration) *Issuer {
	return &Issuer{secret: []byte(secret), ttl: ttl}
}

func (i *Issuer) Issue(ctx context.Context, claims issuer.Claims) (string, error) {
	if claims.Exp == 0 {
		exp := time.Now().Add(i.ttl).Unix()
		if now := time.Now().Unix(); exp <= now {
			exp = now + 1
		}
		claims.Exp = exp
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("hmac issuer: marshal claims: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, i.secret)
	mac.Write([]byte(encoded))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return encoded + "." + sig, nil
}

func (i *Issuer) Verify(ctx context.Context, token string) (*issuer.Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, issuer.ErrInvalidToken
	}
	mac := hmac.New(sha256.New, i.secret)
	mac.Write([]byte(parts[0]))
	sig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, issuer.ErrInvalidToken
	}
	if !hmac.Equal(sig, mac.Sum(nil)) {
		return nil, issuer.ErrInvalidToken
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, issuer.ErrInvalidToken
	}
	var claims issuer.Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, issuer.ErrInvalidToken
	}
	if time.Now().Unix() > claims.Exp {
		return nil, issuer.ErrExpiredToken
	}
	return &claims, nil
}

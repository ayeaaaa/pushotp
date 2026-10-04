package issuer

import (
	"context"
	"errors"
)

var (
	ErrInvalidToken = errors.New("issuer: invalid token")
	ErrExpiredToken = errors.New("issuer: token expired")
)

type Claims struct {
	Receiver string `json:"receiver"`
	Scene    string `json:"scene"`
	Exp      int64  `json:"exp"`
	Nonce    string `json:"nonce"`
}

type Issuer interface {
	Issue(ctx context.Context, claims Claims) (string, error)
	Verify(ctx context.Context, token string) (*Claims, error)
}

package hmac

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"pushotp/issuer"
)

func TestIssueVerifyRoundtrip(t *testing.T) {
	iss := New("secret", time.Hour)
	token, err := iss.Issue(context.Background(), issuer.Claims{Receiver: "admin", Scene: "login"})
	if err != nil {
		t.Fatal(err)
	}
	claims, err := iss.Verify(context.Background(), token)
	if err != nil {
		t.Fatal(err)
	}
	if claims.Receiver != "admin" || claims.Scene != "login" {
		t.Fatalf("claims = %+v", claims)
	}
	if claims.Exp <= time.Now().Unix() {
		t.Fatalf("exp = %d, want in the future", claims.Exp)
	}
}

func TestVerifyTampered(t *testing.T) {
	iss := New("secret", time.Hour)
	token, _ := iss.Issue(context.Background(), issuer.Claims{Receiver: "admin"})
	parts := strings.Split(token, ".")
	tampered := parts[0] + "." + strings.Repeat("A", len(parts[1]))
	if _, err := iss.Verify(context.Background(), tampered); !errors.Is(err, issuer.ErrInvalidToken) {
		t.Fatalf("err = %v, want ErrInvalidToken", err)
	}
	if _, err := New("other", time.Hour).Verify(context.Background(), token); !errors.Is(err, issuer.ErrInvalidToken) {
		t.Fatalf("wrong secret err = %v, want ErrInvalidToken", err)
	}
}

func TestVerifyExpired(t *testing.T) {
	iss := New("secret", time.Hour)
	token, _ := iss.Issue(context.Background(), issuer.Claims{Receiver: "admin", Exp: time.Now().Add(-time.Minute).Unix()})
	if _, err := iss.Verify(context.Background(), token); !errors.Is(err, issuer.ErrExpiredToken) {
		t.Fatalf("err = %v, want ErrExpiredToken", err)
	}
}

func TestIssueSubSecondTTLNotInstantlyExpired(t *testing.T) {
	iss := New("secret", 500*time.Millisecond)
	token, err := iss.Issue(context.Background(), issuer.Claims{Receiver: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := iss.Verify(context.Background(), token); err != nil {
		t.Fatalf("sub-second TTL token should be valid: %v", err)
	}
}

func TestVerifyMalformed(t *testing.T) {
	iss := New("secret", time.Hour)
	for _, token := range []string{"", "abc", "a.b.c", ".", "!!.??"} {
		if _, err := iss.Verify(context.Background(), token); !errors.Is(err, issuer.ErrInvalidToken) {
			t.Fatalf("token %q: err = %v, want ErrInvalidToken", token, err)
		}
	}
}

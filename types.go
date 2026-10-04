package pushotp

import (
	"time"

	"pushotp/issuer"
)

type Ticket struct {
	ID        string
	Receiver  string
	Scene     string
	ExpiresAt time.Time
}

type Claims = issuer.Claims

type SendRequest struct {
	Receiver string
	Scene    string
	TTL      time.Duration
	Length   int
	Data     map[string]string
}

type VerifyRequest struct {
	TicketID string
	Code     string
}

type VerifyResult struct {
	OK        bool
	Token     string
	ExpiresAt time.Time
}

package pushotp

import (
	"time"

	"pushotp/issuer"
	"pushotp/store"
)

type Ticket = store.Ticket

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

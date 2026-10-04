package pushotp

import "errors"

var (
	ErrConfig           = errors.New("pushotp: invalid config")
	ErrReceiverNotFound = errors.New("pushotp: receiver not found")
	ErrCooldown         = errors.New("pushotp: cooldown not elapsed")
	ErrRateLimited      = errors.New("pushotp: hourly quota exceeded")
	ErrTicketNotFound   = errors.New("pushotp: ticket not found")
	ErrExpired          = errors.New("pushotp: ticket expired")
	ErrUsed             = errors.New("pushotp: ticket already used")
	ErrMaxAttempts      = errors.New("pushotp: max attempts exceeded")
	ErrInvalidCode      = errors.New("pushotp: invalid code")
	ErrChannelSend      = errors.New("pushotp: channel send failed")
)

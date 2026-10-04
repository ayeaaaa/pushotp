package httpapi

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"pushotp"
)

type Handler struct {
	v      *pushotp.Verifier
	apiKey string
	logger *slog.Logger
}

func New(v *pushotp.Verifier, apiKey string, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}
	h := &Handler{v: v, apiKey: apiKey, logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/send", h.auth(h.handleSend))
	mux.HandleFunc("POST /api/v1/verify", h.auth(h.handleVerify))
	mux.HandleFunc("GET /api/v1/health", h.handleHealth)
	return mux
}

type sendRequestBody struct {
	Receiver string            `json:"receiver"`
	Scene    string            `json:"scene"`
	TTL      string            `json:"ttl,omitempty"`
	Length   int               `json:"length,omitempty"`
	Data     map[string]string `json:"data,omitempty"`
}

type sendResponseBody struct {
	TicketID  string `json:"ticket_id"`
	ExpiresIn int64  `json:"expires_in"`
}

type verifyRequestBody struct {
	TicketID string `json:"ticket_id"`
	Code     string `json:"code"`
}

type verifyResponseBody struct {
	OK        bool       `json:"ok"`
	Token     string     `json:"token,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

var errorMessages = map[string]string{
	"unauthorized":        "missing or invalid api key",
	"invalid_request":     "invalid request",
	"rate_limited":        "too many requests",
	"ticket_not_found":    "ticket not found",
	"expired":             "verification code expired",
	"used":                "verification code already used",
	"invalid_code":        "invalid verification code",
	"max_attempts":        "too many attempts",
	"receiver_not_found":  "receiver not found",
	"channel_send_failed": "failed to send verification code",
	"internal":            "internal error",
}

func (h *Handler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.apiKey != "" {
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(h.apiKey)) != 1 {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
		}
		next(w, r)
	}
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) handleSend(w http.ResponseWriter, r *http.Request) {
	var body sendRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	var ttl time.Duration
	if body.TTL != "" {
		parsed, err := time.ParseDuration(body.TTL)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request")
			return
		}
		ttl = parsed
	}
	tk, err := h.v.Send(r.Context(), pushotp.SendRequest{
		Receiver: body.Receiver,
		Scene:    body.Scene,
		TTL:      ttl,
		Length:   body.Length,
		Data:     body.Data,
	})
	if err != nil {
		h.writeVerifierError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sendResponseBody{
		TicketID:  tk.ID,
		ExpiresIn: int64(time.Until(tk.ExpiresAt).Seconds()),
	})
}

func (h *Handler) handleVerify(w http.ResponseWriter, r *http.Request) {
	var body verifyRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	res, err := h.v.Verify(r.Context(), pushotp.VerifyRequest{TicketID: body.TicketID, Code: body.Code})
	if err != nil {
		h.writeVerifierError(w, err)
		return
	}
	out := verifyResponseBody{OK: res.OK, Token: res.Token}
	if !res.ExpiresAt.IsZero() {
		out.ExpiresAt = &res.ExpiresAt
	}
	writeJSON(w, http.StatusOK, out)
}

func (h *Handler) writeVerifierError(w http.ResponseWriter, err error) {
	status, code := mapError(err)
	if status >= http.StatusInternalServerError {
		h.logger.Error("request failed", "code", code, "err", err)
	} else {
		h.logger.Warn("request rejected", "code", code, "err", err)
	}
	writeError(w, status, code)
}

func mapError(err error) (int, string) {
	switch {
	case errors.Is(err, pushotp.ErrCooldown), errors.Is(err, pushotp.ErrRateLimited):
		return http.StatusTooManyRequests, "rate_limited"
	case errors.Is(err, pushotp.ErrTicketNotFound):
		return http.StatusNotFound, "ticket_not_found"
	case errors.Is(err, pushotp.ErrExpired):
		return http.StatusGone, "expired"
	case errors.Is(err, pushotp.ErrUsed):
		return http.StatusGone, "used"
	case errors.Is(err, pushotp.ErrInvalidCode):
		return http.StatusBadRequest, "invalid_code"
	case errors.Is(err, pushotp.ErrMaxAttempts):
		return http.StatusBadRequest, "max_attempts"
	case errors.Is(err, pushotp.ErrReceiverNotFound):
		return http.StatusBadRequest, "receiver_not_found"
	case errors.Is(err, pushotp.ErrChannelSend):
		return http.StatusBadGateway, "channel_send_failed"
	default:
		return http.StatusInternalServerError, "internal"
	}
}

func writeError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, errorBody{Error: errorDetail{Code: code, Message: errorMessages[code]}})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

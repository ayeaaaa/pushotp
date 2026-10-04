package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"pushotp"
	"pushotp/channel"
)

var codeRe = regexp.MustCompile(`\d{4,8}`)

type fakeChannel struct {
	fn func(ctx context.Context, target channel.Target, msg channel.Message) error
}

func (f *fakeChannel) Name() string { return "fake" }

func (f *fakeChannel) Send(ctx context.Context, target channel.Target, msg channel.Message) error {
	return f.fn(ctx, target, msg)
}

func registerFake(t *testing.T, fn func(context.Context, channel.Target, channel.Message) error) string {
	t.Helper()
	name := "fake-" + strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
	if fn == nil {
		fn = func(context.Context, channel.Target, channel.Message) error { return nil }
	}
	channel.Register(name, func() channel.Channel { return &fakeChannel{fn: fn} })
	return name
}

func baseConfig(chName string) pushotp.Config {
	return pushotp.Config{
		Code: pushotp.CodeConfig{
			Length:      6,
			TTL:         time.Minute,
			Cooldown:    time.Nanosecond,
			MaxAttempts: 5,
			MaxPerHour:  100,
		},
		Receivers: []pushotp.ReceiverConfig{{Name: "admin", Channel: chName}},
	}
}

func newHandler(t *testing.T, cfg pushotp.Config, apiKey string) http.Handler {
	t.Helper()
	v, err := pushotp.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return New(v, apiKey, nil)
}

func doJSON(t *testing.T, h http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decodeErrorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %q: %v", rec.Body.String(), err)
	}
	return body.Error.Code
}

func TestHealth(t *testing.T) {
	name := registerFake(t, nil)
	h := newHandler(t, baseConfig(name), "")
	rec := doJSON(t, h, http.MethodGet, "/api/v1/health", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestSendVerifyRoundtrip(t *testing.T) {
	var content string
	name := registerFake(t, func(_ context.Context, _ channel.Target, msg channel.Message) error {
		content = msg.Content
		return nil
	})
	h := newHandler(t, baseConfig(name), "")

	rec := doJSON(t, h, http.MethodPost, "/api/v1/send", map[string]any{"receiver": "admin", "scene": "login"}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("send status = %d body = %s", rec.Code, rec.Body)
	}
	var sendResp struct {
		TicketID  string `json:"ticket_id"`
		ExpiresIn int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &sendResp); err != nil {
		t.Fatal(err)
	}
	if sendResp.TicketID == "" || sendResp.ExpiresIn <= 0 {
		t.Fatalf("unexpected send response: %+v", sendResp)
	}
	code := codeRe.FindString(content)
	if code == "" {
		t.Fatalf("no code in message %q", content)
	}
	rec = doJSON(t, h, http.MethodPost, "/api/v1/verify", map[string]string{"ticket_id": sendResp.TicketID, "code": code}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify status = %d body = %s", rec.Code, rec.Body)
	}
	var verifyResp struct {
		OK bool `json:"ok"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &verifyResp); err != nil {
		t.Fatal(err)
	}
	if !verifyResp.OK {
		t.Fatalf("verify response = %s", rec.Body)
	}
}

func TestAPIKey(t *testing.T) {
	name := registerFake(t, nil)
	h := newHandler(t, baseConfig(name), "secret")
	body := map[string]any{"receiver": "admin"}

	rec := doJSON(t, h, http.MethodPost, "/api/v1/send", body, nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no key status = %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, "/api/v1/send", body, map[string]string{"Authorization": "Bearer wrong"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("wrong key status = %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodPost, "/api/v1/send", body, map[string]string{"Authorization": "Bearer secret"})
	if rec.Code != http.StatusOK {
		t.Fatalf("correct key status = %d body = %s", rec.Code, rec.Body)
	}
	rec = doJSON(t, h, http.MethodGet, "/api/v1/health", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("health must not require key, status = %d", rec.Code)
	}
}

func TestErrorMapping(t *testing.T) {
	name := registerFake(t, func(context.Context, channel.Target, channel.Message) error {
		return errors.New("boom")
	})
	h := newHandler(t, baseConfig(name), "")

	rec := doJSON(t, h, http.MethodPost, "/api/v1/send", map[string]any{"receiver": "ghost"}, nil)
	if rec.Code != http.StatusBadRequest || decodeErrorCode(t, rec) != "receiver_not_found" {
		t.Fatalf("receiver: status = %d body = %s", rec.Code, rec.Body)
	}
	rec = doJSON(t, h, http.MethodPost, "/api/v1/send", map[string]any{"receiver": "admin"}, nil)
	if rec.Code != http.StatusBadGateway || decodeErrorCode(t, rec) != "channel_send_failed" {
		t.Fatalf("channel: status = %d body = %s", rec.Code, rec.Body)
	}
	rec = doJSON(t, h, http.MethodPost, "/api/v1/verify", map[string]string{"ticket_id": "nope", "code": "123456"}, nil)
	if rec.Code != http.StatusNotFound || decodeErrorCode(t, rec) != "ticket_not_found" {
		t.Fatalf("ticket: status = %d body = %s", rec.Code, rec.Body)
	}
}

func TestCooldownMapping(t *testing.T) {
	name := registerFake(t, nil)
	cfg := baseConfig(name)
	cfg.Code.Cooldown = time.Hour
	h := newHandler(t, cfg, "")
	body := map[string]any{"receiver": "admin"}
	if rec := doJSON(t, h, http.MethodPost, "/api/v1/send", body, nil); rec.Code != http.StatusOK {
		t.Fatalf("first send status = %d", rec.Code)
	}
	rec := doJSON(t, h, http.MethodPost, "/api/v1/send", body, nil)
	if rec.Code != http.StatusTooManyRequests || decodeErrorCode(t, rec) != "rate_limited" {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body)
	}
}

func TestInvalidJSON(t *testing.T) {
	name := registerFake(t, nil)
	h := newHandler(t, baseConfig(name), "")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/send", strings.NewReader("{"))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || decodeErrorCode(t, rec) != "invalid_request" {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body)
	}
}

package pushotp

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

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

type sentMessage struct {
	target channel.Target
	msg    channel.Message
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

func testConfig(chName string) Config {
	return Config{
		Storage: StorageConfig{Type: "memory"},
		Code: CodeConfig{
			Length:      6,
			TTL:         time.Minute,
			Cooldown:    time.Nanosecond,
			MaxAttempts: 5,
			MaxPerHour:  100,
		},
		Receivers: []ReceiverConfig{{Name: "admin", Channel: chName}},
	}
}

func newTestVerifier(t *testing.T, cfg Config) *Verifier {
	t.Helper()
	v, err := New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return v
}

func TestSendSuccess(t *testing.T) {
	var sent *sentMessage
	name := registerFake(t, func(_ context.Context, target channel.Target, msg channel.Message) error {
		sent = &sentMessage{target: target, msg: msg}
		return nil
	})
	v := newTestVerifier(t, testConfig(name))

	tk, err := v.Send(context.Background(), SendRequest{Receiver: "admin", Scene: "login"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if tk.ID == "" || !tk.ExpiresAt.After(time.Now()) {
		t.Fatalf("unexpected ticket: %+v", tk)
	}
	if sent == nil || sent.target.Receiver != "admin" {
		t.Fatalf("channel did not receive message: %+v", sent)
	}
	if code := codeRe.FindString(sent.msg.Content); len(code) != 6 {
		t.Fatalf("message %q has no 6-digit code", sent.msg.Content)
	}
}

func TestSendUnknownReceiver(t *testing.T) {
	name := registerFake(t, nil)
	v := newTestVerifier(t, testConfig(name))
	if _, err := v.Send(context.Background(), SendRequest{Receiver: "nobody"}); !errors.Is(err, ErrReceiverNotFound) {
		t.Fatalf("err = %v, want ErrReceiverNotFound", err)
	}
}

func TestSendCooldown(t *testing.T) {
	name := registerFake(t, nil)
	cfg := testConfig(name)
	cfg.Code.Cooldown = time.Hour
	v := newTestVerifier(t, cfg)
	if _, err := v.Send(context.Background(), SendRequest{Receiver: "admin"}); err != nil {
		t.Fatalf("first Send: %v", err)
	}
	if _, err := v.Send(context.Background(), SendRequest{Receiver: "admin"}); !errors.Is(err, ErrCooldown) {
		t.Fatalf("err = %v, want ErrCooldown", err)
	}
}

func TestSendRateLimited(t *testing.T) {
	name := registerFake(t, nil)
	cfg := testConfig(name)
	cfg.Code.MaxPerHour = 1
	v := newTestVerifier(t, cfg)
	if _, err := v.Send(context.Background(), SendRequest{Receiver: "admin"}); err != nil {
		t.Fatalf("first Send: %v", err)
	}
	if _, err := v.Send(context.Background(), SendRequest{Receiver: "admin"}); !errors.Is(err, ErrRateLimited) {
		t.Fatalf("err = %v, want ErrRateLimited", err)
	}
}

func TestSendChannelFailureDeletesTicket(t *testing.T) {
	name := registerFake(t, func(context.Context, channel.Target, channel.Message) error {
		return errors.New("boom")
	})
	v := newTestVerifier(t, testConfig(name))
	if _, err := v.Send(context.Background(), SendRequest{Receiver: "admin"}); !errors.Is(err, ErrChannelSend) {
		t.Fatalf("err = %v, want ErrChannelSend", err)
	}
	count, err := v.store.CountRecent(context.Background(), "admin", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 after failed send", count)
	}
}

func TestSendChannelFailureCleansUpWithCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	name := registerFake(t, func(context.Context, channel.Target, channel.Message) error {
		cancel()
		return errors.New("send failed")
	})
	cfg := testConfig(name)
	cfg.Storage = StorageConfig{Type: "sqlite", SQLite: SQLiteConfig{Path: t.TempDir() + "/test.db"}}
	v := newTestVerifier(t, cfg)
	if _, err := v.Send(ctx, SendRequest{Receiver: "admin"}); !errors.Is(err, ErrChannelSend) {
		t.Fatalf("err = %v, want ErrChannelSend", err)
	}
	count, err := v.store.CountRecent(context.Background(), "admin", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("count = %d, want 0 after failed send with canceled ctx", count)
	}
	last, err := v.store.LastSentAt(context.Background(), "admin")
	if err != nil {
		t.Fatal(err)
	}
	if !last.IsZero() {
		t.Fatalf("last = %v, want zero", last)
	}
}

func TestSendCustomLengthAndData(t *testing.T) {
	var content string
	name := registerFake(t, func(_ context.Context, _ channel.Target, msg channel.Message) error {
		content = msg.Content
		return nil
	})
	cfg := testConfig(name)
	cfg.Receivers[0].Template = "【{app}】{code}"
	v := newTestVerifier(t, cfg)
	if _, err := v.Send(context.Background(), SendRequest{
		Receiver: "admin",
		Length:   4,
		Data:     map[string]string{"app": "面板"},
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !strings.HasPrefix(content, "【面板】") {
		t.Fatalf("content = %q", content)
	}
	if code := codeRe.FindString(content); len(code) != 4 {
		t.Fatalf("content %q has no 4-digit code", content)
	}
}

func TestVerifySuccess(t *testing.T) {
	var sent sentMessage
	name := registerFake(t, func(_ context.Context, target channel.Target, msg channel.Message) error {
		sent = sentMessage{target: target, msg: msg}
		return nil
	})
	v := newTestVerifier(t, testConfig(name))
	tk, err := v.Send(context.Background(), SendRequest{Receiver: "admin", Scene: "login"})
	if err != nil {
		t.Fatal(err)
	}
	code := codeRe.FindString(sent.msg.Content)
	res, err := v.Verify(context.Background(), VerifyRequest{TicketID: tk.ID, Code: code})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !res.OK {
		t.Fatal("expected OK")
	}
	if res.Token != "" {
		t.Fatalf("token = %q, want empty when issuer disabled", res.Token)
	}
}

func TestVerifyWrongCode(t *testing.T) {
	var sent sentMessage
	name := registerFake(t, func(_ context.Context, _ channel.Target, msg channel.Message) error {
		sent = sentMessage{msg: msg}
		return nil
	})
	v := newTestVerifier(t, testConfig(name))
	tk, err := v.Send(context.Background(), SendRequest{Receiver: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	code := codeRe.FindString(sent.msg.Content)
	wrong := "000000"
	if wrong == code {
		wrong = "111111"
	}
	if _, err := v.Verify(context.Background(), VerifyRequest{TicketID: tk.ID, Code: wrong}); !errors.Is(err, ErrInvalidCode) {
		t.Fatalf("err = %v, want ErrInvalidCode", err)
	}
	stored, err := v.store.GetTicket(context.Background(), tk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Attempts != 1 {
		t.Fatalf("attempts = %d, want 1", stored.Attempts)
	}
}

func TestVerifyMaxAttempts(t *testing.T) {
	var sent sentMessage
	name := registerFake(t, func(_ context.Context, _ channel.Target, msg channel.Message) error {
		sent = sentMessage{msg: msg}
		return nil
	})
	cfg := testConfig(name)
	cfg.Code.MaxAttempts = 2
	v := newTestVerifier(t, cfg)
	tk, err := v.Send(context.Background(), SendRequest{Receiver: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	code := codeRe.FindString(sent.msg.Content)
	wrong := "000000"
	if wrong == code {
		wrong = "111111"
	}
	for i := 0; i < 2; i++ {
		if _, err := v.Verify(context.Background(), VerifyRequest{TicketID: tk.ID, Code: wrong}); !errors.Is(err, ErrInvalidCode) {
			t.Fatalf("attempt %d: err = %v, want ErrInvalidCode", i+1, err)
		}
	}
	if _, err := v.Verify(context.Background(), VerifyRequest{TicketID: tk.ID, Code: code}); !errors.Is(err, ErrMaxAttempts) {
		t.Fatalf("err = %v, want ErrMaxAttempts", err)
	}
}

func TestVerifyUsed(t *testing.T) {
	var sent sentMessage
	name := registerFake(t, func(_ context.Context, _ channel.Target, msg channel.Message) error {
		sent = sentMessage{msg: msg}
		return nil
	})
	v := newTestVerifier(t, testConfig(name))
	tk, err := v.Send(context.Background(), SendRequest{Receiver: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	code := codeRe.FindString(sent.msg.Content)
	if _, err := v.Verify(context.Background(), VerifyRequest{TicketID: tk.ID, Code: code}); err != nil {
		t.Fatal(err)
	}
	if _, err := v.Verify(context.Background(), VerifyRequest{TicketID: tk.ID, Code: code}); !errors.Is(err, ErrUsed) {
		t.Fatalf("err = %v, want ErrUsed", err)
	}
}

func TestVerifyExpired(t *testing.T) {
	var sent sentMessage
	name := registerFake(t, func(_ context.Context, _ channel.Target, msg channel.Message) error {
		sent = sentMessage{msg: msg}
		return nil
	})
	v := newTestVerifier(t, testConfig(name))
	base := time.Now()
	v.now = func() time.Time { return base }
	tk, err := v.Send(context.Background(), SendRequest{Receiver: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	code := codeRe.FindString(sent.msg.Content)
	v.now = func() time.Time { return base.Add(10 * time.Minute) }
	if _, err := v.Verify(context.Background(), VerifyRequest{TicketID: tk.ID, Code: code}); !errors.Is(err, ErrExpired) {
		t.Fatalf("err = %v, want ErrExpired", err)
	}
}

func TestVerifyUnknownTicket(t *testing.T) {
	name := registerFake(t, nil)
	v := newTestVerifier(t, testConfig(name))
	if _, err := v.Verify(context.Background(), VerifyRequest{TicketID: "nope", Code: "123456"}); !errors.Is(err, ErrTicketNotFound) {
		t.Fatalf("err = %v, want ErrTicketNotFound", err)
	}
}

func TestVerifyIssuerToken(t *testing.T) {
	var sent sentMessage
	name := registerFake(t, func(_ context.Context, _ channel.Target, msg channel.Message) error {
		sent = sentMessage{msg: msg}
		return nil
	})
	cfg := testConfig(name)
	cfg.Issuer = IssuerConfig{Enabled: true, Secret: "s3cret", TTL: time.Hour}
	v := newTestVerifier(t, cfg)
	tk, err := v.Send(context.Background(), SendRequest{Receiver: "admin", Scene: "login"})
	if err != nil {
		t.Fatal(err)
	}
	code := codeRe.FindString(sent.msg.Content)
	res, err := v.Verify(context.Background(), VerifyRequest{TicketID: tk.ID, Code: code})
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if res.Token == "" || res.ExpiresAt.IsZero() {
		t.Fatalf("expected token, got %+v", res)
	}
	claims, err := v.VerifyToken(res.Token)
	if err != nil {
		t.Fatalf("VerifyToken: %v", err)
	}
	if claims.Receiver != "admin" || claims.Scene != "login" {
		t.Fatalf("claims = %+v", claims)
	}
	if _, err := v.VerifyToken(res.Token + "x"); err == nil {
		t.Fatal("tampered token must fail")
	}
}

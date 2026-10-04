package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ayeaaaa/pushotp/channel"
)

func TestSendSuccess(t *testing.T) {
	var path string
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	c := &Channel{BaseURL: srv.URL, Client: srv.Client()}
	err := c.Send(context.Background(), channel.Target{
		Receiver: "me",
		Config:   map[string]string{"bot_token": "123:abc", "chat_id": "42"},
	}, channel.Message{Title: "验证码", Content: "code 654321"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if path != "/bot123:abc/sendMessage" {
		t.Fatalf("path = %q", path)
	}
	if got["chat_id"] != "42" {
		t.Fatalf("chat_id = %q", got["chat_id"])
	}
}

func TestSendMissingCredentials(t *testing.T) {
	c := New()
	if err := c.Send(context.Background(), channel.Target{}, channel.Message{Content: "x"}); err == nil {
		t.Fatal("expected error for missing credentials")
	}
}

func TestSendAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"description":"chat not found"}`))
	}))
	defer srv.Close()

	c := &Channel{BaseURL: srv.URL, Client: srv.Client()}
	err := c.Send(context.Background(), channel.Target{
		Config: map[string]string{"bot_token": "t", "chat_id": "1"},
	}, channel.Message{Content: "x"})
	if err == nil {
		t.Fatal("expected api error")
	}
}

func TestSendErrorDoesNotLeakToken(t *testing.T) {
	c := &Channel{BaseURL: "http://127.0.0.1:1", Client: &http.Client{Timeout: time.Second}}
	err := c.Send(context.Background(), channel.Target{
		Config: map[string]string{"bot_token": "123456:SUPER_SECRET", "chat_id": "1"},
	}, channel.Message{Content: "x"})
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "SUPER_SECRET") {
		t.Fatalf("token leaked in error: %v", err)
	}
}

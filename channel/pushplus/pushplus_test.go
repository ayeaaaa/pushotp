package pushplus

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ayeaaaa/pushotp/channel"
)

func TestSendSuccess(t *testing.T) {
	var got map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/send" {
			t.Errorf("path = %q, want /send", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &got)
		_, _ = w.Write([]byte(`{"code":200,"msg":"ok"}`))
	}))
	defer srv.Close()

	c := &Channel{BaseURL: srv.URL, Client: srv.Client()}
	err := c.Send(context.Background(), channel.Target{
		Receiver: "admin",
		Config:   map[string]string{"token": "tok-1"},
	}, channel.Message{Title: "验证码", Content: "code 123456"})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	if got["token"] != "tok-1" || got["title"] != "验证码" || got["content"] != "code 123456" {
		t.Fatalf("unexpected request body: %#v", got)
	}
}

func TestSendMissingToken(t *testing.T) {
	c := New()
	err := c.Send(context.Background(), channel.Target{Receiver: "admin"}, channel.Message{Content: "x"})
	if err == nil {
		t.Fatal("expected error for missing token")
	}
}

func TestSendAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":500,"msg":"bad token"}`))
	}))
	defer srv.Close()

	c := &Channel{BaseURL: srv.URL, Client: srv.Client()}
	err := c.Send(context.Background(), channel.Target{
		Config: map[string]string{"token": "tok"},
	}, channel.Message{Content: "x"})
	if err == nil {
		t.Fatal("expected api error")
	}
}

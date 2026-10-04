package template

import (
	"strings"
	"testing"
)

func TestRenderDefault(t *testing.T) {
	out := Render("", Data{Code: "123456", TTL: "5m"})
	if !strings.Contains(out, "123456") {
		t.Fatalf("output %q missing code", out)
	}
}

func TestRenderBuiltins(t *testing.T) {
	out := Render("{receiver}|{scene}|{code}|{ttl}", Data{
		Code: "0001", TTL: "10m", Scene: "login", Receiver: "admin",
	})
	if out != "admin|login|0001|10m" {
		t.Fatalf("output = %q", out)
	}
}

func TestRenderExtra(t *testing.T) {
	out := Render("【{app}】{code}", Data{Code: "42", Extra: map[string]string{"app": "面板"}})
	if out != "【面板】42" {
		t.Fatalf("output = %q", out)
	}
}

func TestRenderUnknownPlaceholderKept(t *testing.T) {
	out := Render("x {unknown} {code}", Data{Code: "7"})
	if out != "x {unknown} 7" {
		t.Fatalf("output = %q", out)
	}
}

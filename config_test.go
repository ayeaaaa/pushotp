package pushotp

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func validConfig() Config {
	return Config{
		Receivers: []ReceiverConfig{{
			Name:     "admin",
			Channel:  "pushplus",
			Pushplus: PushplusConfig{Token: "tok"},
		}},
	}.WithDefaults()
}

func TestWithDefaults(t *testing.T) {
	cfg := Config{}.WithDefaults()
	if cfg.Server.Addr != ":8080" {
		t.Fatalf("addr = %q", cfg.Server.Addr)
	}
	if cfg.Storage.Type != "memory" {
		t.Fatalf("storage = %q", cfg.Storage.Type)
	}
	if cfg.Code.Length != 6 || cfg.Code.TTL != 5*time.Minute || cfg.Code.Cooldown != time.Minute {
		t.Fatalf("code defaults = %+v", cfg.Code)
	}
	if cfg.Code.MaxAttempts != 5 || cfg.Code.MaxPerHour != 10 {
		t.Fatalf("limits = %+v", cfg.Code)
	}
	if cfg.Issuer.TTL != 24*time.Hour {
		t.Fatalf("issuer ttl = %v", cfg.Issuer.TTL)
	}
	if again := cfg.WithDefaults(); !reflect.DeepEqual(cfg, again) {
		t.Fatal("WithDefaults must be idempotent")
	}
}

func TestValidateOK(t *testing.T) {
	if err := validConfig().Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidateErrors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
	}{
		{"no receivers", func(c *Config) { c.Receivers = nil }},
		{"duplicate name", func(c *Config) { c.Receivers = append(c.Receivers, c.Receivers[0]) }},
		{"unknown channel", func(c *Config) { c.Receivers[0].Channel = "nope" }},
		{"missing pushplus token", func(c *Config) { c.Receivers[0].Pushplus.Token = "" }},
		{"missing telegram creds", func(c *Config) { c.Receivers[0] = ReceiverConfig{Name: "me", Channel: "telegram"} }},
		{"issuer without secret", func(c *Config) { c.Issuer.Enabled = true }},
		{"issuer non-positive ttl", func(c *Config) {
			c.Issuer.Enabled = true
			c.Issuer.Secret = "s"
			c.Issuer.TTL = -time.Minute
		}},
		{"bad length", func(c *Config) { c.Code.Length = 3 }},
		{"bad storage", func(c *Config) { c.Storage.Type = "redis" }},
		{"sqlite without path", func(c *Config) { c.Storage.Type = "sqlite" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)
			if err := cfg.Validate(); !errors.Is(err, ErrConfig) {
				t.Fatalf("err = %v, want ErrConfig", err)
			}
		})
	}
}

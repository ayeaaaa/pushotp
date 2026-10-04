package main

import (
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

const sampleYAML = `
server:
  addr: ":9090"
  api_key: "k"
storage:
  type: sqlite
  sqlite:
    path: ./x.db
code:
  length: 5
  ttl: 10m
  cooldown: 30s
  max_attempts: 3
  max_per_hour: 4
issuer:
  enabled: true
  secret: "s"
  ttl: 2h
receivers:
  - name: admin
    channel: pushplus
    pushplus:
      token: "tok"
    template: "【{app}】{code}"
  - name: me
    channel: telegram
    telegram:
      bot_token: "bt"
      chat_id: "42"
`

func TestFileConfigToConfig(t *testing.T) {
	var fc fileConfig
	if err := yaml.Unmarshal([]byte(sampleYAML), &fc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cfg, err := fc.toConfig()
	if err != nil {
		t.Fatalf("toConfig: %v", err)
	}
	if cfg.Server.Addr != ":9090" || cfg.Server.APIKey != "k" {
		t.Fatalf("server = %+v", cfg.Server)
	}
	if cfg.Storage.Type != "sqlite" || cfg.Storage.SQLite.Path != "./x.db" {
		t.Fatalf("storage = %+v", cfg.Storage)
	}
	if cfg.Code.Length != 5 || cfg.Code.TTL != 10*time.Minute || cfg.Code.Cooldown != 30*time.Second {
		t.Fatalf("code = %+v", cfg.Code)
	}
	if cfg.Code.MaxAttempts != 3 || cfg.Code.MaxPerHour != 4 {
		t.Fatalf("limits = %+v", cfg.Code)
	}
	if !cfg.Issuer.Enabled || cfg.Issuer.Secret != "s" || cfg.Issuer.TTL != 2*time.Hour {
		t.Fatalf("issuer = %+v", cfg.Issuer)
	}
	if len(cfg.Receivers) != 2 {
		t.Fatalf("receivers = %+v", cfg.Receivers)
	}
	if cfg.Receivers[0].Pushplus.Token != "tok" || cfg.Receivers[0].Template == "" {
		t.Fatalf("receiver[0] = %+v", cfg.Receivers[0])
	}
	if cfg.Receivers[1].Telegram.BotToken != "bt" || cfg.Receivers[1].Telegram.ChatID != "42" {
		t.Fatalf("receiver[1] = %+v", cfg.Receivers[1])
	}
}

func TestFileConfigInvalidDuration(t *testing.T) {
	var fc fileConfig
	if err := yaml.Unmarshal([]byte("code:\n  ttl: nope\n"), &fc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, err := fc.toConfig(); err == nil {
		t.Fatal("expected error for invalid duration")
	}
}

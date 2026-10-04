package main

import (
	"bytes"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/ayeaaaa/pushotp"
)

func decodeConfig(raw []byte) (fileConfig, error) {
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	var fc fileConfig
	if err := dec.Decode(&fc); err != nil {
		return fileConfig{}, err
	}
	return fc, nil
}

type fileConfig struct {
	Server    serverConfig     `yaml:"server"`
	Storage   storageConfig    `yaml:"storage"`
	Code      codeConfig       `yaml:"code"`
	Issuer    issuerConfig     `yaml:"issuer"`
	Receivers []receiverConfig `yaml:"receivers"`
}

type serverConfig struct {
	Addr   string `yaml:"addr"`
	APIKey string `yaml:"api_key"`
}

type storageConfig struct {
	Type   string `yaml:"type"`
	SQLite struct {
		Path string `yaml:"path"`
	} `yaml:"sqlite"`
}

type codeConfig struct {
	Length      int    `yaml:"length"`
	TTL         string `yaml:"ttl"`
	Cooldown    string `yaml:"cooldown"`
	MaxAttempts int    `yaml:"max_attempts"`
	MaxPerHour  int    `yaml:"max_per_hour"`
}

type issuerConfig struct {
	Enabled bool   `yaml:"enabled"`
	Secret  string `yaml:"secret"`
	TTL     string `yaml:"ttl"`
}

type receiverConfig struct {
	Name     string `yaml:"name"`
	Channel  string `yaml:"channel"`
	Pushplus struct {
		Token string `yaml:"token"`
	} `yaml:"pushplus"`
	Telegram struct {
		BotToken string `yaml:"bot_token"`
		ChatID   string `yaml:"chat_id"`
	} `yaml:"telegram"`
	Template string            `yaml:"template"`
	Config   map[string]string `yaml:"config"`
}

func (f fileConfig) toConfig() (pushotp.Config, error) {
	cfg := pushotp.Config{
		Server: pushotp.ServerConfig{Addr: f.Server.Addr, APIKey: f.Server.APIKey},
		Storage: pushotp.StorageConfig{
			Type:   f.Storage.Type,
			SQLite: pushotp.SQLiteConfig{Path: f.Storage.SQLite.Path},
		},
		Code: pushotp.CodeConfig{
			Length:      f.Code.Length,
			MaxAttempts: f.Code.MaxAttempts,
			MaxPerHour:  f.Code.MaxPerHour,
		},
		Issuer: pushotp.IssuerConfig{
			Enabled: f.Issuer.Enabled,
			Secret:  f.Issuer.Secret,
		},
	}
	var err error
	if cfg.Code.TTL, err = parseDuration(f.Code.TTL); err != nil {
		return cfg, fmt.Errorf("code.ttl: %w", err)
	}
	if cfg.Code.Cooldown, err = parseDuration(f.Code.Cooldown); err != nil {
		return cfg, fmt.Errorf("code.cooldown: %w", err)
	}
	if cfg.Issuer.TTL, err = parseDuration(f.Issuer.TTL); err != nil {
		return cfg, fmt.Errorf("issuer.ttl: %w", err)
	}
	for _, r := range f.Receivers {
		cfg.Receivers = append(cfg.Receivers, pushotp.ReceiverConfig{
			Name:     r.Name,
			Channel:  r.Channel,
			Pushplus: pushotp.PushplusConfig{Token: r.Pushplus.Token},
			Telegram: pushotp.TelegramConfig{BotToken: r.Telegram.BotToken, ChatID: r.Telegram.ChatID},
			Template: r.Template,
			Config:   r.Config,
		})
	}
	return cfg, nil
}

func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, nil
	}
	return time.ParseDuration(s)
}

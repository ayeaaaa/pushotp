package pushotp

import (
	"fmt"
	"time"

	"pushotp/channel"
)

type Config struct {
	Server    ServerConfig     `yaml:"server"`
	Storage   StorageConfig    `yaml:"storage"`
	Code      CodeConfig       `yaml:"code"`
	Issuer    IssuerConfig     `yaml:"issuer"`
	Receivers []ReceiverConfig `yaml:"receivers"`
}

type ServerConfig struct {
	Addr   string `yaml:"addr"`
	APIKey string `yaml:"api_key"`
}

type StorageConfig struct {
	Type   string       `yaml:"type"`
	SQLite SQLiteConfig `yaml:"sqlite"`
}

type SQLiteConfig struct {
	Path string `yaml:"path"`
}

type CodeConfig struct {
	Length      int           `yaml:"length"`
	TTL         time.Duration `yaml:"ttl"`
	Cooldown    time.Duration `yaml:"cooldown"`
	MaxAttempts int           `yaml:"max_attempts"`
	MaxPerHour  int           `yaml:"max_per_hour"`
}

type IssuerConfig struct {
	Enabled bool          `yaml:"enabled"`
	Secret  string        `yaml:"secret"`
	TTL     time.Duration `yaml:"ttl"`
}

type PushplusConfig struct {
	Token string `yaml:"token"`
}

type TelegramConfig struct {
	BotToken string `yaml:"bot_token"`
	ChatID   string `yaml:"chat_id"`
}

type ReceiverConfig struct {
	Name     string         `yaml:"name"`
	Channel  string         `yaml:"channel"`
	Pushplus PushplusConfig `yaml:"pushplus"`
	Telegram TelegramConfig `yaml:"telegram"`
	Template string         `yaml:"template"`
}

func (c Config) WithDefaults() Config {
	if c.Server.Addr == "" {
		c.Server.Addr = ":8080"
	}
	if c.Storage.Type == "" {
		c.Storage.Type = "memory"
	}
	if c.Code.Length == 0 {
		c.Code.Length = 6
	}
	if c.Code.TTL == 0 {
		c.Code.TTL = 5 * time.Minute
	}
	if c.Code.Cooldown == 0 {
		c.Code.Cooldown = time.Minute
	}
	if c.Code.MaxAttempts == 0 {
		c.Code.MaxAttempts = 5
	}
	if c.Code.MaxPerHour == 0 {
		c.Code.MaxPerHour = 10
	}
	if c.Issuer.TTL == 0 {
		c.Issuer.TTL = 24 * time.Hour
	}
	return c
}

func (c Config) Validate() error {
	if len(c.Receivers) == 0 {
		return fmt.Errorf("%w: at least one receiver is required", ErrConfig)
	}
	if c.Code.Length < 4 || c.Code.Length > 8 {
		return fmt.Errorf("%w: code length %d out of range [4,8]", ErrConfig, c.Code.Length)
	}
	if c.Code.TTL <= 0 || c.Code.Cooldown <= 0 {
		return fmt.Errorf("%w: code ttl and cooldown must be positive", ErrConfig)
	}
	if c.Code.MaxAttempts <= 0 || c.Code.MaxPerHour <= 0 {
		return fmt.Errorf("%w: max_attempts and max_per_hour must be positive", ErrConfig)
	}
	if c.Issuer.Enabled && c.Issuer.Secret == "" {
		return fmt.Errorf("%w: issuer enabled but secret is empty", ErrConfig)
	}
	if c.Issuer.Enabled && c.Issuer.TTL <= 0 {
		return fmt.Errorf("%w: issuer ttl must be positive", ErrConfig)
	}
	if c.Storage.Type != "memory" && c.Storage.Type != "sqlite" {
		return fmt.Errorf("%w: unknown storage type %q", ErrConfig, c.Storage.Type)
	}
	if c.Storage.Type == "sqlite" && c.Storage.SQLite.Path == "" {
		return fmt.Errorf("%w: sqlite path is required", ErrConfig)
	}
	seen := map[string]bool{}
	for _, r := range c.Receivers {
		if r.Name == "" {
			return fmt.Errorf("%w: receiver name is required", ErrConfig)
		}
		if seen[r.Name] {
			return fmt.Errorf("%w: duplicate receiver %q", ErrConfig, r.Name)
		}
		seen[r.Name] = true
		if !channel.Registered(r.Channel) {
			return fmt.Errorf("%w: receiver %q uses unknown channel %q", ErrConfig, r.Name, r.Channel)
		}
		switch r.Channel {
		case "pushplus":
			if r.Pushplus.Token == "" {
				return fmt.Errorf("%w: receiver %q missing pushplus token", ErrConfig, r.Name)
			}
		case "telegram":
			if r.Telegram.BotToken == "" || r.Telegram.ChatID == "" {
				return fmt.Errorf("%w: receiver %q missing telegram bot_token or chat_id", ErrConfig, r.Name)
			}
		}
	}
	return nil
}

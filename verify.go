package pushotp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"pushotp/channel"
	"pushotp/issuer"
	hmacissuer "pushotp/issuer/hmac"
	"pushotp/store"
	"pushotp/store/memory"
	"pushotp/store/sqlite"
	"pushotp/template"
)

type Verifier struct {
	cfg      Config
	store    store.Store
	issuer   issuer.Issuer
	channels map[string]channel.Channel
	now      func() time.Time
	logger   *slog.Logger
	sendMu   sync.Mutex
}

func New(cfg Config) (*Verifier, error) {
	cfg = cfg.WithDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	st, err := buildStore(cfg.Storage)
	if err != nil {
		return nil, err
	}
	chans := make(map[string]channel.Channel, len(cfg.Receivers))
	for _, r := range cfg.Receivers {
		ch, err := channel.New(r.Channel)
		if err != nil {
			_ = st.Close()
			return nil, fmt.Errorf("%w: %w", ErrConfig, err)
		}
		chans[r.Name] = ch
	}
	var iss issuer.Issuer
	if cfg.Issuer.Enabled {
		iss = hmacissuer.New(cfg.Issuer.Secret, cfg.Issuer.TTL)
	}
	return &Verifier{
		cfg:      cfg,
		store:    st,
		issuer:   iss,
		channels: chans,
		now:      time.Now,
		logger:   slog.Default(),
	}, nil
}

func buildStore(cfg StorageConfig) (store.Store, error) {
	switch cfg.Type {
	case "memory":
		return memory.New(), nil
	case "sqlite":
		st, err := sqlite.New(cfg.SQLite.Path)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrConfig, err)
		}
		return st, nil
	default:
		return nil, fmt.Errorf("%w: unknown storage type %q", ErrConfig, cfg.Type)
	}
}

func (v *Verifier) Close() error {
	if v.store == nil {
		return nil
	}
	return v.store.Close()
}

func (v *Verifier) receiver(name string) (ReceiverConfig, bool) {
	for _, r := range v.cfg.Receivers {
		if r.Name == name {
			return r, true
		}
	}
	return ReceiverConfig{}, false
}

func (v *Verifier) channelConfig(r ReceiverConfig) map[string]string {
	switch r.Channel {
	case "pushplus":
		return map[string]string{"token": r.Pushplus.Token}
	case "telegram":
		return map[string]string{"bot_token": r.Telegram.BotToken, "chat_id": r.Telegram.ChatID}
	default:
		return nil
	}
}

func (v *Verifier) Send(ctx context.Context, req SendRequest) (*Ticket, error) {
	v.sendMu.Lock()
	defer v.sendMu.Unlock()

	rc, ok := v.receiver(req.Receiver)
	if !ok {
		return nil, ErrReceiverNotFound
	}
	now := v.now()
	last, err := v.store.LastSentAt(ctx, req.Receiver)
	if err != nil {
		return nil, err
	}
	if !last.IsZero() && now.Sub(last) < v.cfg.Code.Cooldown {
		return nil, ErrCooldown
	}
	count, err := v.store.CountRecent(ctx, req.Receiver, time.Hour)
	if err != nil {
		return nil, err
	}
	if count >= v.cfg.Code.MaxPerHour {
		return nil, ErrRateLimited
	}
	length := v.cfg.Code.Length
	if req.Length > 0 {
		length = req.Length
	}
	code, err := generateCode(length)
	if err != nil {
		return nil, err
	}
	salt, err := randomBytes(16)
	if err != nil {
		return nil, err
	}
	id, err := newID()
	if err != nil {
		return nil, err
	}
	ttl := v.cfg.Code.TTL
	if req.TTL > 0 {
		ttl = req.TTL
	}
	t := store.Ticket{
		ID:        id,
		Receiver:  req.Receiver,
		Scene:     req.Scene,
		CodeHash:  hashCode(salt, code),
		Salt:      salt,
		ExpiresAt: now.Add(ttl),
		CreatedAt: now,
	}
	if err := v.store.SaveTicket(ctx, t); err != nil {
		return nil, err
	}
	content := template.Render(rc.Template, template.Data{
		Code:     code,
		TTL:      ttl.String(),
		Scene:    req.Scene,
		Receiver: req.Receiver,
		Extra:    req.Data,
	})
	msg := channel.Message{Title: "验证码", Content: content}
	target := channel.Target{Receiver: req.Receiver, Config: v.channelConfig(rc)}
	if err := v.channels[req.Receiver].Send(ctx, target, msg); err != nil {
		if delErr := v.store.DeleteTicket(context.WithoutCancel(ctx), id); delErr != nil {
			v.logger.Warn("delete failed-send ticket", "err", delErr)
		}
		return nil, fmt.Errorf("%w: %w", ErrChannelSend, err)
	}
	return &Ticket{ID: t.ID, Receiver: t.Receiver, Scene: t.Scene, ExpiresAt: t.ExpiresAt}, nil
}

func (v *Verifier) Verify(ctx context.Context, req VerifyRequest) (*VerifyResult, error) {
	t, err := v.store.GetTicket(ctx, req.TicketID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	now := v.now()
	if t.Used {
		return nil, ErrUsed
	}
	if now.After(t.ExpiresAt) {
		return nil, ErrExpired
	}
	if t.Attempts >= v.cfg.Code.MaxAttempts {
		return nil, ErrMaxAttempts
	}
	if !equalHash(t.CodeHash, hashCode(t.Salt, req.Code)) {
		if _, err := v.store.IncrAttempt(ctx, t.ID); err != nil {
			return nil, err
		}
		return nil, ErrInvalidCode
	}
	if err := v.store.MarkUsed(ctx, t.ID); err != nil {
		return nil, err
	}
	res := &VerifyResult{OK: true}
	if v.issuer != nil {
		nonce, err := newID()
		if err != nil {
			return nil, err
		}
		claims := issuer.Claims{
			Receiver: t.Receiver,
			Scene:    t.Scene,
			Exp:      now.Add(v.cfg.Issuer.TTL).Unix(),
			Nonce:    nonce,
		}
		token, err := v.issuer.Issue(ctx, claims)
		if err != nil {
			return nil, err
		}
		res.Token = token
		res.ExpiresAt = time.Unix(claims.Exp, 0)
	}
	return res, nil
}

func (v *Verifier) VerifyToken(token string) (*Claims, error) {
	if v.issuer == nil {
		return nil, fmt.Errorf("%w: issuer is disabled", ErrConfig)
	}
	return v.issuer.Verify(context.Background(), token)
}

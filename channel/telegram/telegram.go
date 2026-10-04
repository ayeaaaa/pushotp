package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"pushotp/channel"
)

const DefaultBaseURL = "https://api.telegram.org"

type Channel struct {
	BaseURL string
	Client  *http.Client
}

func New() *Channel {
	return &Channel{
		BaseURL: DefaultBaseURL,
		Client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Channel) Name() string { return "telegram" }

type sendRequest struct {
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

type sendResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func sanitizeURLError(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

func (c *Channel) Send(ctx context.Context, target channel.Target, msg channel.Message) error {
	token := target.Config["bot_token"]
	chatID := target.Config["chat_id"]
	if token == "" || chatID == "" {
		return fmt.Errorf("telegram: missing bot_token or chat_id for receiver %q", target.Receiver)
	}
	text := msg.Content
	if msg.Title != "" {
		text = msg.Title + "\n" + msg.Content
	}
	payload, err := json.Marshal(sendRequest{ChatID: chatID, Text: text})
	if err != nil {
		return fmt.Errorf("telegram: marshal request: %w", err)
	}
	url := c.BaseURL + "/bot" + token + "/sendMessage"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("telegram: build request: %w", sanitizeURLError(err))
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram: http: %w", sanitizeURLError(err))
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return fmt.Errorf("telegram: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram: http status %d: %s", resp.StatusCode, data)
	}
	var sr sendResponse
	if err := json.Unmarshal(data, &sr); err != nil {
		return fmt.Errorf("telegram: decode response: %w", err)
	}
	if !sr.OK {
		return fmt.Errorf("telegram: api error: %s", sr.Description)
	}
	return nil
}

func init() {
	channel.Register("telegram", func() channel.Channel { return New() })
}

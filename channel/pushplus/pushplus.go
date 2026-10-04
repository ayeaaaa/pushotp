package pushplus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"pushotp/channel"
)

const DefaultBaseURL = "https://www.pushplus.plus"

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

func (c *Channel) Name() string { return "pushplus" }

type sendRequest struct {
	Token    string `json:"token"`
	Title    string `json:"title"`
	Content  string `json:"content"`
	Template string `json:"template"`
}

type sendResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func (c *Channel) Send(ctx context.Context, target channel.Target, msg channel.Message) error {
	token := target.Config["token"]
	if token == "" {
		return fmt.Errorf("pushplus: missing token for receiver %q", target.Receiver)
	}
	payload, err := json.Marshal(sendRequest{
		Token:    token,
		Title:    msg.Title,
		Content:  msg.Content,
		Template: "txt",
	})
	if err != nil {
		return fmt.Errorf("pushplus: marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/send", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("pushplus: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.Client.Do(req)
	if err != nil {
		return fmt.Errorf("pushplus: http: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return fmt.Errorf("pushplus: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("pushplus: http status %d: %s", resp.StatusCode, data)
	}
	var sr sendResponse
	if err := json.Unmarshal(data, &sr); err != nil {
		return fmt.Errorf("pushplus: decode response: %w", err)
	}
	if sr.Code != 200 {
		return fmt.Errorf("pushplus: api code %d: %s", sr.Code, sr.Msg)
	}
	return nil
}

func init() {
	channel.Register("pushplus", func() channel.Channel { return New() })
}

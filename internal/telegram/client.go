package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type Message struct {
	ID   int    `json:"message_id"`
	From *User  `json:"from"`
	Chat Chat   `json:"chat"`
	Text string `json:"text"`
}

type Update struct {
	ID      int64    `json:"update_id"`
	Message *Message `json:"message"`
}

type Client struct {
	token      string
	baseURL    string
	httpClient *http.Client
}

func New(token string) (*Client, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("Telegram bot token is empty")
	}
	return &Client{
		token: token, baseURL: "https://api.telegram.org",
		httpClient: &http.Client{Timeout: 70 * time.Second},
	}, nil
}

func newWithHTTPClient(token, baseURL string, httpClient *http.Client) *Client {
	return &Client{token: token, baseURL: strings.TrimRight(baseURL, "/"), httpClient: httpClient}
}

func (c *Client) GetUpdates(ctx context.Context, offset int64) ([]Update, error) {
	var updates []Update
	err := c.call(ctx, "getUpdates", map[string]any{
		"offset": offset, "timeout": 50, "allowed_updates": []string{"message"},
	}, &updates)
	return updates, err
}

func (c *Client) GetMe(ctx context.Context) (User, error) {
	var user User
	if err := c.call(ctx, "getMe", map[string]any{}, &user); err != nil {
		return User{}, err
	}
	if !user.IsBot || user.ID == 0 {
		return User{}, fmt.Errorf("Telegram getMe returned an invalid bot user")
	}
	return user, nil
}

func (c *Client) SetWebhook(ctx context.Context, url, secret string) error {
	url = strings.TrimSpace(url)
	secret = strings.TrimSpace(secret)
	if !strings.HasPrefix(url, "https://") {
		return fmt.Errorf("Telegram webhook URL must use https://")
	}
	if secret == "" {
		return fmt.Errorf("Telegram webhook secret is empty")
	}
	return c.call(ctx, "setWebhook", map[string]any{
		"url": url, "secret_token": secret,
		"allowed_updates": []string{"message"}, "max_connections": 1,
	}, nil)
}

func (c *Client) SendMessage(ctx context.Context, chatID int64, text string) error {
	return c.SendMessageWithKeyboard(ctx, chatID, text, nil)
}

func (c *Client) SendMessageWithKeyboard(ctx context.Context, chatID int64, text string, rows [][]string) error {
	payload := map[string]any{"chat_id": chatID, "text": text}
	if len(rows) > 0 {
		keyboard := make([][]map[string]string, 0, len(rows))
		for _, row := range rows {
			buttons := make([]map[string]string, 0, len(row))
			for _, label := range row {
				if strings.TrimSpace(label) != "" {
					buttons = append(buttons, map[string]string{"text": label})
				}
			}
			if len(buttons) > 0 {
				keyboard = append(keyboard, buttons)
			}
		}
		if len(keyboard) > 0 {
			payload["reply_markup"] = map[string]any{
				"keyboard": keyboard, "resize_keyboard": true, "is_persistent": true,
			}
		}
	}
	return c.call(ctx, "sendMessage", payload, nil)
}

func (c *Client) call(ctx context.Context, method string, payload, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode Telegram %s request: %w", method, err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/bot"+c.token+"/"+method, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Telegram %s request: %w", method, err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("Telegram %s request failed", method)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read Telegram %s response: %w", method, err)
	}
	var envelope struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Description string          `json:"description"`
	}
	if err := json.Unmarshal(responseBody, &envelope); err != nil {
		return fmt.Errorf("decode Telegram %s response: %w", method, err)
	}
	if !envelope.OK {
		return fmt.Errorf("Telegram %s failed: %s", method, envelope.Description)
	}
	if result != nil && len(envelope.Result) > 0 {
		if err := json.Unmarshal(envelope.Result, result); err != nil {
			return fmt.Errorf("decode Telegram %s result: %w", method, err)
		}
	}
	return nil
}

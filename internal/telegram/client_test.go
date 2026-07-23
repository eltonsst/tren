package telegram

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func fakeHTTPClient(t *testing.T, handler func(*http.Request) string) *http.Client {
	t.Helper()
	return &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/json"}},
			Body:       io.NopCloser(strings.NewReader(handler(request))),
			Request:    request,
		}, nil
	})}
}

func TestGetUpdates(t *testing.T) {
	client := newWithHTTPClient("secret-token", "https://telegram.test", fakeHTTPClient(t, func(request *http.Request) string {
		if request.URL.Path != "/botsecret-token/getUpdates" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["offset"] != float64(7) {
			t.Fatalf("offset = %#v", body["offset"])
		}
		return `{"ok":true,"result":[{"update_id":7,"message":{"message_id":3,"from":{"id":99},"chat":{"id":99},"text":"/current"}}]}`
	}))
	updates, err := client.GetUpdates(context.Background(), 7)
	if err != nil {
		t.Fatalf("GetUpdates() error = %v", err)
	}
	if len(updates) != 1 || updates[0].Message.Text != "/current" {
		t.Fatalf("updates = %#v", updates)
	}
}

func TestGetMe(t *testing.T) {
	client := newWithHTTPClient("secret-token", "https://telegram.test", fakeHTTPClient(t, func(request *http.Request) string {
		if request.URL.Path != "/botsecret-token/getMe" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		return `{"ok":true,"result":{"id":123,"is_bot":true,"first_name":"Tren","username":"tren_test_bot"}}`
	}))
	user, err := client.GetMe(context.Background())
	if err != nil || user.ID != 123 || !user.IsBot || user.Username != "tren_test_bot" {
		t.Fatalf("user = %#v, err = %v", user, err)
	}
}

func TestSetWebhook(t *testing.T) {
	client := newWithHTTPClient("secret-token", "https://telegram.test", fakeHTTPClient(t, func(request *http.Request) string {
		if request.URL.Path != "/botsecret-token/setWebhook" {
			t.Fatalf("path = %q", request.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["url"] != "https://tren.test/telegram/webhook" || body["secret_token"] != "webhook-secret" || body["max_connections"] != float64(1) {
			t.Fatalf("body = %#v", body)
		}
		return `{"ok":true,"result":true}`
	}))
	if err := client.SetWebhook(context.Background(), "https://tren.test/telegram/webhook", "webhook-secret"); err != nil {
		t.Fatalf("SetWebhook() error = %v", err)
	}
}

func TestSendMessage(t *testing.T) {
	client := newWithHTTPClient("secret-token", "https://telegram.test", fakeHTTPClient(t, func(request *http.Request) string {
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["chat_id"] != float64(99) || body["text"] != "Workout A" {
			t.Fatalf("body = %#v", body)
		}
		return `{"ok":true,"result":{"message_id":4}}`
	}))
	if err := client.SendMessage(context.Background(), 99, "Workout A"); err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
}

func TestSendMessageWithPersistentKeyboard(t *testing.T) {
	client := newWithHTTPClient("secret-token", "https://telegram.test", fakeHTTPClient(t, func(request *http.Request) string {
		var body map[string]any
		if err := json.NewDecoder(request.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		markup, ok := body["reply_markup"].(map[string]any)
		if !ok || markup["resize_keyboard"] != true || markup["is_persistent"] != true {
			t.Fatalf("reply_markup = %#v", body["reply_markup"])
		}
		rows, ok := markup["keyboard"].([]any)
		if !ok || len(rows) != 2 {
			t.Fatalf("keyboard = %#v", markup["keyboard"])
		}
		firstRow, ok := rows[0].([]any)
		if !ok || len(firstRow) != 2 {
			t.Fatalf("first row = %#v", rows[0])
		}
		firstButton, ok := firstRow[0].(map[string]any)
		if !ok || firstButton["text"] != "/current" {
			t.Fatalf("first button = %#v", firstRow[0])
		}
		return `{"ok":true,"result":{"message_id":4}}`
	}))
	if err := client.SendMessageWithKeyboard(context.Background(), 99, "Choose a command", [][]string{
		{"/current", "/start"},
		{"/undo", "/howto"},
	}); err != nil {
		t.Fatalf("SendMessageWithKeyboard() error = %v", err)
	}
}

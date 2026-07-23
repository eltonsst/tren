package hosted

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/eltonsst/tren/internal/telegram"
)

type fakeBot struct {
	calls int
}

func (bot *fakeBot) Handle(_ context.Context, update telegram.Update) (int64, string, error) {
	bot.calls++
	return update.Message.Chat.ID, "Workout A", nil
}

func (*fakeBot) CommandKeyboard(context.Context) ([][]string, error) {
	return [][]string{{"/current", "/start"}}, nil
}

type fakeTelegram struct {
	chatID int64
	reply  string
}

func (client *fakeTelegram) SendMessageWithKeyboard(_ context.Context, chatID int64, reply string, _ [][]string) error {
	client.chatID, client.reply = chatID, reply
	return nil
}

func testApp(t *testing.T) (*App, *fakeBot, *fakeTelegram) {
	t.Helper()
	bot := &fakeBot{}
	client := &fakeTelegram{}
	app, err := New(bot, client, http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		_, _ = writer.Write([]byte("dashboard"))
	}), Options{WebhookSecret: "webhook-secret", AllowedTelegramUserID: 99, DashboardUser: "elton", DashboardPassword: "dashboard-secret"})
	if err != nil {
		t.Fatal(err)
	}
	return app, bot, client
}

func TestHealthCheckIsPublic(t *testing.T) {
	app, _, _ := testApp(t)
	response := httptest.NewRecorder()
	app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if response.Code != http.StatusOK || response.Body.String() != "ok\n" {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
}

func TestDashboardRequiresBasicAuthentication(t *testing.T) {
	app, _, _ := testApp(t)
	response := httptest.NewRecorder()
	app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", response.Code)
	}

	request := httptest.NewRequest(http.MethodGet, "/", nil)
	request.SetBasicAuth("elton", "dashboard-secret")
	response = httptest.NewRecorder()
	app.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Body.String() != "dashboard" {
		t.Fatalf("status = %d, body = %q", response.Code, response.Body.String())
	}
}

func TestWebhookRejectsMissingSecretWithoutInvokingBot(t *testing.T) {
	app, bot, _ := testApp(t)
	request := httptest.NewRequest(http.MethodPost, WebhookPath, strings.NewReader(`{"update_id":1}`))
	response := httptest.NewRecorder()
	app.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden || bot.calls != 0 {
		t.Fatalf("status = %d, calls = %d", response.Code, bot.calls)
	}
}

func TestWebhookProcessesUpdateAndSendsReply(t *testing.T) {
	app, bot, client := testApp(t)
	body := `{"update_id":7,"message":{"message_id":3,"from":{"id":99},"chat":{"id":99},"text":"/current"}}`
	request := httptest.NewRequest(http.MethodPost, WebhookPath, strings.NewReader(body))
	request.Header.Set(telegramSecretHeader, "webhook-secret")
	response := httptest.NewRecorder()
	app.ServeHTTP(response, request)
	if response.Code != http.StatusOK || bot.calls != 1 || client.chatID != 99 || client.reply != "Workout A" {
		t.Fatalf("status = %d, calls = %d, client = %#v", response.Code, bot.calls, client)
	}
}

func TestWebhookIgnoresUnauthorizedTelegramUserBeforeBotOrSheets(t *testing.T) {
	app, bot, client := testApp(t)
	body := `{"update_id":8,"message":{"message_id":4,"from":{"id":666},"chat":{"id":666},"text":"/current"}}`
	request := httptest.NewRequest(http.MethodPost, WebhookPath, strings.NewReader(body))
	request.Header.Set(telegramSecretHeader, "webhook-secret")
	response := httptest.NewRecorder()
	app.ServeHTTP(response, request)
	if response.Code != http.StatusOK || bot.calls != 0 || client.chatID != 0 {
		t.Fatalf("status = %d, calls = %d, client = %#v", response.Code, bot.calls, client)
	}
}

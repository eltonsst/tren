package hosted

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/eltonsst/tren/internal/telegram"
)

const (
	WebhookPath          = "/telegram/webhook"
	telegramSecretHeader = "X-Telegram-Bot-Api-Secret-Token"
	maxWebhookBody       = 1 << 20
)

type Bot interface {
	Handle(context.Context, telegram.Update) (int64, string, error)
	CommandKeyboard(context.Context) ([][]string, error)
}

type Telegram interface {
	SendMessageWithKeyboard(context.Context, int64, string, [][]string) error
}

type Options struct {
	WebhookSecret         string
	AllowedTelegramUserID int64
	DashboardUser         string
	DashboardPassword     string
	Logger                *log.Logger
}

type App struct {
	bot                   Bot
	telegram              Telegram
	dashboard             http.Handler
	webhookSecret         string
	allowedTelegramUserID int64
	dashboardUser         string
	dashboardPassword     string
	logger                *log.Logger
	updateMu              sync.Mutex
}

func New(bot Bot, telegramClient Telegram, dashboard http.Handler, options Options) (*App, error) {
	if bot == nil || telegramClient == nil || dashboard == nil {
		return nil, fmt.Errorf("hosted bot, Telegram client, and dashboard are required")
	}
	if strings.TrimSpace(options.WebhookSecret) == "" {
		return nil, fmt.Errorf("webhook secret is required")
	}
	if strings.TrimSpace(options.DashboardPassword) == "" {
		return nil, fmt.Errorf("dashboard password is required")
	}
	username := strings.TrimSpace(options.DashboardUser)
	if username == "" {
		username = "tren"
	}
	logger := options.Logger
	if logger == nil {
		logger = log.Default()
	}
	return &App{
		bot: bot, telegram: telegramClient, dashboard: dashboard,
		webhookSecret:         strings.TrimSpace(options.WebhookSecret),
		allowedTelegramUserID: options.AllowedTelegramUserID,
		dashboardUser:         username, dashboardPassword: options.DashboardPassword,
		logger: logger,
	}, nil
}

func (app *App) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	switch request.URL.Path {
	case "/healthz":
		if request.Method != http.MethodGet {
			http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		writer.Header().Set("Content-Type", "text/plain; charset=utf-8")
		writer.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(writer, "ok\n")
	case WebhookPath:
		app.serveWebhook(writer, request)
	default:
		app.serveDashboard(writer, request)
	}
}

func (app *App) serveWebhook(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodPost {
		http.Error(writer, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !secureEqual(request.Header.Get(telegramSecretHeader), app.webhookSecret) {
		http.Error(writer, "forbidden", http.StatusForbidden)
		return
	}
	request.Body = http.MaxBytesReader(writer, request.Body, maxWebhookBody)
	decoder := json.NewDecoder(request.Body)
	var update telegram.Update
	if err := decoder.Decode(&update); err != nil {
		http.Error(writer, "invalid Telegram update", http.StatusBadRequest)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		http.Error(writer, "invalid Telegram update", http.StatusBadRequest)
		return
	}
	if app.allowedTelegramUserID != 0 && (update.Message == nil || update.Message.From == nil || update.Message.From.ID != app.allowedTelegramUserID) {
		writer.WriteHeader(http.StatusOK)
		return
	}

	app.updateMu.Lock()
	defer app.updateMu.Unlock()
	chatID, reply, err := app.bot.Handle(request.Context(), update)
	if err != nil {
		app.logger.Printf("Telegram update %d failed: %v", update.ID, err)
		reply = "I couldn't process that command in Google Sheets."
	}
	if reply != "" && chatID != 0 {
		keyboard, keyboardErr := app.bot.CommandKeyboard(request.Context())
		if keyboardErr != nil {
			app.logger.Printf("Telegram keyboard state failed: %v", keyboardErr)
		}
		if sendErr := app.telegram.SendMessageWithKeyboard(request.Context(), chatID, reply, keyboard); sendErr != nil {
			app.logger.Printf("Telegram reply failed: %v", sendErr)
			http.Error(writer, "Telegram reply failed", http.StatusBadGateway)
			return
		}
	}
	writer.WriteHeader(http.StatusOK)
}

func (app *App) serveDashboard(writer http.ResponseWriter, request *http.Request) {
	username, password, ok := request.BasicAuth()
	if !ok || !secureEqual(username, app.dashboardUser) || !secureEqual(password, app.dashboardPassword) {
		writer.Header().Set("WWW-Authenticate", `Basic realm="Tren dashboard", charset="UTF-8"`)
		http.Error(writer, "authentication required", http.StatusUnauthorized)
		return
	}
	app.dashboard.ServeHTTP(writer, request)
}

func secureEqual(left, right string) bool {
	if len(left) != len(right) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func Serve(ctx context.Context, address string, handler http.Handler) error {
	listener, err := net.Listen("tcp", strings.TrimSpace(address))
	if err != nil {
		return fmt.Errorf("start Tren service on %s: %w", address, err)
	}
	server := &http.Server{
		Handler: handler, ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second,
		IdleTimeout: 60 * time.Second,
	}
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		<-ctx.Done()
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownContext)
	}()
	log.Printf("Tren service listening on %s", listener.Addr())
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		<-shutdownDone
		return nil
	}
	return err
}

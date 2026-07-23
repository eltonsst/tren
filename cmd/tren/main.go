package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/eltonsst/tren/internal/config"
	"github.com/eltonsst/tren/internal/dashboard"
	"github.com/eltonsst/tren/internal/doctor"
	"github.com/eltonsst/tren/internal/hosted"
	"github.com/eltonsst/tren/internal/onboarding"
	"github.com/eltonsst/tren/internal/readonlybot"
	"github.com/eltonsst/tren/internal/sheetscheck"
	"github.com/eltonsst/tren/internal/stats"
	"github.com/eltonsst/tren/internal/telegram"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "tren:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string) error {
	if len(arguments) == 0 {
		return usageError()
	}
	if containsArgument(arguments[1:], "--demo") {
		switch arguments[0] {
		case "stats":
			return runStats(ctx, nil, arguments[1:])
		case "dashboard":
			return runDashboard(ctx, nil, arguments[1:], strings.TrimSpace(os.Getenv(config.EnvDashboardAddress)))
		}
	}
	configuration, err := config.Load()
	if err != nil {
		return err
	}
	switch arguments[0] {
	case "setup":
		if len(arguments) != 1 {
			return usageError()
		}
		_, err := onboarding.Setup(configuration, os.Stdin, os.Stdout)
		return err
	case "doctor":
		if len(arguments) != 1 {
			return usageError()
		}
		return runDoctor(ctx, configuration)
	case "check":
		if len(arguments) != 1 {
			return usageError()
		}
		if err := configuration.ValidateSheets(); err != nil {
			return err
		}
		checker, err := sheetscheck.NewReadOnly(ctx, configuration.SpreadsheetID, configuration.GoogleCredentialsSource())
		if err != nil {
			return err
		}
		return runCheck(ctx, checker)
	case "bot":
		if len(arguments) != 1 {
			return usageError()
		}
		if err := configuration.ValidateBot(); err != nil {
			return err
		}
		checker, err := sheetscheck.NewReadWrite(ctx, configuration.SpreadsheetID, configuration.GoogleCredentialsSource())
		if err != nil {
			return err
		}
		return runBot(ctx, checker, configuration)
	case "serve":
		if len(arguments) != 1 {
			return usageError()
		}
		if err := configuration.ValidateServe(); err != nil {
			return err
		}
		checker, err := sheetscheck.NewReadWrite(ctx, configuration.SpreadsheetID, configuration.GoogleCredentialsSource())
		if err != nil {
			return err
		}
		return runServe(ctx, checker, configuration)
	case "stats":
		if err := configuration.ValidateSheets(); err != nil {
			return err
		}
		checker, err := sheetscheck.NewReadOnly(ctx, configuration.SpreadsheetID, configuration.GoogleCredentialsSource())
		if err != nil {
			return err
		}
		return runStats(ctx, checker, arguments[1:])
	case "dashboard":
		if err := configuration.ValidateSheets(); err != nil {
			return err
		}
		checker, err := sheetscheck.NewReadOnly(ctx, configuration.SpreadsheetID, configuration.GoogleCredentialsSource())
		if err != nil {
			return err
		}
		return runDashboard(ctx, checker, arguments[1:], configuration.DashboardAddress)
	default:
		return usageError()
	}
}

func runDashboard(ctx context.Context, checker *sheetscheck.Checker, arguments []string, address string) error {
	demo, noOpen := false, false
	var queryParts []string
	for _, argument := range arguments {
		switch argument {
		case "--demo":
			demo = true
		case "--no-open":
			noOpen = true
		default:
			if strings.HasPrefix(argument, "--") {
				return fmt.Errorf("unknown dashboard option %q", argument)
			}
			queryParts = append(queryParts, argument)
		}
	}
	var dataset stats.Dataset
	if demo {
		dataset = stats.OfflineDemo(time.Now())
	} else {
		if checker == nil {
			return fmt.Errorf("Google Sheets client is unavailable")
		}
		var err error
		dataset, err = checker.StatsDataset(ctx)
		if err != nil {
			return err
		}
	}
	return dashboard.Serve(ctx, dataset, strings.Join(queryParts, " "), dashboard.ServeOptions{
		Address: address, OpenBrowser: !noOpen,
	})
}

func runStats(ctx context.Context, checker *sheetscheck.Checker, arguments []string) error {
	demo := false
	var queryParts []string
	for _, argument := range arguments {
		if argument == "--plain" {
			continue
		}
		if argument == "--demo" {
			demo = true
			continue
		}
		if strings.HasPrefix(argument, "--") {
			return fmt.Errorf("unknown stats option %q", argument)
		}
		queryParts = append(queryParts, argument)
	}
	query := strings.Join(queryParts, " ")
	var dataset stats.Dataset
	if demo {
		dataset = stats.OfflineDemo(time.Now())
	} else {
		if checker == nil {
			return fmt.Errorf("Google Sheets client is unavailable")
		}
		var err error
		dataset, err = checker.StatsDataset(ctx)
		if err != nil {
			return err
		}
	}
	if strings.TrimSpace(query) == "" {
		fmt.Print(stats.RenderCatalogue(dataset))
	} else {
		report, err := stats.BuildReport(dataset, query)
		if err != nil {
			return err
		}
		fmt.Print(stats.RenderReport(dataset, report))
	}
	return nil
}

func containsArgument(arguments []string, target string) bool {
	for _, argument := range arguments {
		if argument == target {
			return true
		}
	}
	return false
}

func runCheck(ctx context.Context, checker *sheetscheck.Checker) error {
	summary, err := checker.Check(ctx)
	if err != nil {
		return err
	}
	fmt.Printf("Sheets OK — schema %s.\n", summary.SchemaVersion)
	fmt.Printf("Active program: %s.\n", summary.ActiveProgramVersion)
	fmt.Printf("Next workout: %s — %d exercises.\n", summary.NextWorkout, summary.NextWorkoutExercises)
	fmt.Printf("Exercise catalogue: %d exercises.\n", summary.ExerciseCount)
	return nil
}

func runDoctor(ctx context.Context, settings config.Settings) error {
	results := doctor.Local(settings)

	if err := settings.ValidateSheets(); err != nil {
		results = append(results, doctor.Result{Name: "Google Sheets", Status: doctor.Skipped, Detail: err.Error()})
	} else {
		checker, err := sheetscheck.NewReadOnly(ctx, settings.SpreadsheetID, settings.GoogleCredentialsSource())
		if err != nil {
			results = append(results, doctor.Result{Name: "Google Sheets", Status: doctor.Failed, Detail: err.Error()})
		} else if summary, err := checker.Check(ctx); err != nil {
			results = append(results, doctor.Result{Name: "Google Sheets", Status: doctor.Failed, Detail: err.Error()})
		} else {
			results = append(results, doctor.Result{
				Name: "Google Sheets", Status: doctor.OK,
				Detail: fmt.Sprintf("schema %s; %s; next Workout %s", summary.SchemaVersion, summary.ActiveProgramVersion, summary.NextWorkout),
			})
		}
	}

	if strings.TrimSpace(settings.TelegramTokenFile) == "" && strings.TrimSpace(settings.TelegramToken) == "" {
		results = append(results, doctor.Result{Name: "Telegram API", Status: doctor.Skipped, Detail: "token file is not configured"})
	} else if token, err := settings.TelegramTokenValue(); err != nil {
		results = append(results, doctor.Result{Name: "Telegram API", Status: doctor.Skipped, Detail: err.Error()})
	} else if client, err := telegram.New(token); err != nil {
		results = append(results, doctor.Result{Name: "Telegram API", Status: doctor.Failed, Detail: err.Error()})
	} else if user, err := client.GetMe(ctx); err != nil {
		results = append(results, doctor.Result{Name: "Telegram API", Status: doctor.Failed, Detail: err.Error()})
	} else {
		results = append(results, doctor.Result{Name: "Telegram API", Status: doctor.OK, Detail: "@" + user.Username})
	}

	fmt.Println("Tren doctor")
	for _, result := range results {
		fmt.Printf("  [%-7s] %-20s %s\n", result.Status, result.Name, result.Detail)
	}
	if doctor.HasFailures(results) {
		return fmt.Errorf("doctor found configuration problems")
	}
	fmt.Println("Ready.")
	return nil
}

func runBot(ctx context.Context, checker *sheetscheck.Checker, settings config.Settings) error {
	token, err := settings.TelegramTokenValue()
	if err != nil {
		return err
	}
	client, err := telegram.New(token)
	if err != nil {
		return err
	}
	handler := readonlybot.New(checker)
	log.Println("Tren is listening. Use the command keyboard in Telegram; press Ctrl-C to stop.")

	var offset int64
	for {
		if ctx.Err() != nil {
			return nil
		}
		updates, err := client.GetUpdates(ctx, offset)
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return nil
			}
			log.Printf("Telegram polling failed: %v", err)
			if !wait(ctx, 3*time.Second) {
				return nil
			}
			continue
		}
		for _, update := range updates {
			chatID, reply, err := handler.Handle(ctx, update)
			if err != nil {
				log.Printf("Telegram update %d failed: %v", update.ID, err)
				reply = "I couldn't process that command in Google Sheets."
			}
			if reply != "" && chatID != 0 {
				keyboard, keyboardErr := handler.CommandKeyboard(ctx)
				if keyboardErr != nil {
					log.Printf("Telegram keyboard state failed: %v", keyboardErr)
				}
				if err := client.SendMessageWithKeyboard(ctx, chatID, reply, keyboard); err != nil {
					log.Printf("Telegram reply failed: %v", err)
				}
			}
			if update.ID >= offset {
				offset = update.ID + 1
			}
		}
	}
}

func runServe(ctx context.Context, checker *sheetscheck.Checker, settings config.Settings) error {
	token, err := settings.TelegramTokenValue()
	if err != nil {
		return err
	}
	client, err := telegram.New(token)
	if err != nil {
		return err
	}
	dashboardHandler, err := dashboard.NewDynamic(checker.StatsDataset, "")
	if err != nil {
		return err
	}
	workout, err := checker.CurrentWorkout(ctx)
	if err != nil {
		return fmt.Errorf("load hosted Telegram authorization: %w", err)
	}
	if workout.AuthorizedTelegramUserID == 0 {
		return fmt.Errorf("Config authorized_telegram_user_id must be set before hosted mode starts")
	}
	app, err := hosted.New(readonlybot.New(checker), client, dashboardHandler, hosted.Options{
		WebhookSecret: settings.WebhookSecret, AllowedTelegramUserID: workout.AuthorizedTelegramUserID,
		DashboardUser: settings.DashboardUser, DashboardPassword: settings.DashboardPassword,
	})
	if err != nil {
		return err
	}
	if settings.PublicURL != "" {
		webhookURL := settings.PublicURL + hosted.WebhookPath
		registerContext, cancel := context.WithTimeout(ctx, 15*time.Second)
		registerErr := client.SetWebhook(registerContext, webhookURL, settings.WebhookSecret)
		cancel()
		if registerErr != nil {
			return fmt.Errorf("register Telegram webhook: %w", registerErr)
		}
		log.Printf("Telegram webhook registered at %s", webhookURL)
	} else {
		log.Printf("%s is unset; serving locally without changing the Telegram webhook", config.EnvPublicURL)
	}
	return hosted.Serve(ctx, settings.ServeAddress(), app)
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func usageError() error {
	return fmt.Errorf("usage:\n  tren setup\n  tren doctor\n  tren check\n  tren bot\n  tren serve\n  tren dashboard [exercise] [--demo] [--no-open]\n  tren stats [exercise] [--demo]")
}

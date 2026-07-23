package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const (
	EnvConfigFile            = "TREN_CONFIG"
	EnvSpreadsheetID         = "TREN_SHEET_ID"
	EnvGoogleCredentials     = "TREN_GOOGLE_CREDENTIALS"
	EnvGoogleCredentialsJSON = "TREN_GOOGLE_CREDENTIALS_JSON"
	EnvTelegramTokenFile     = "TREN_TELEGRAM_TOKEN_FILE"
	EnvTelegramToken         = "TREN_TELEGRAM_TOKEN"
	EnvDashboardAddress      = "TREN_DASHBOARD_ADDR"
	EnvDashboardUser         = "TREN_DASHBOARD_USER"
	EnvDashboardPassword     = "TREN_DASHBOARD_PASSWORD"
	EnvWebhookSecret         = "TREN_WEBHOOK_SECRET"
	EnvPublicURL             = "TREN_PUBLIC_URL"
	EnvPort                  = "PORT"
	EnvRailwayPublicDomain   = "RAILWAY_PUBLIC_DOMAIN"
)

type Settings struct {
	SpreadsheetID         string `json:"spreadsheet_id"`
	GoogleCredentialsFile string `json:"google_credentials_file"`
	TelegramTokenFile     string `json:"telegram_token_file"`
	DashboardAddress      string `json:"dashboard_address"`
	GoogleCredentialsJSON string `json:"-"`
	TelegramToken         string `json:"-"`
	DashboardUser         string `json:"-"`
	DashboardPassword     string `json:"-"`
	WebhookSecret         string `json:"-"`
	PublicURL             string `json:"-"`
	Port                  string `json:"-"`
	ConfigFile            string `json:"-"`
	ConfigFileFound       bool   `json:"-"`
}

type fileSettings struct {
	SpreadsheetID         string `json:"spreadsheet_id"`
	GoogleCredentialsFile string `json:"google_credentials_file"`
	TelegramTokenFile     string `json:"telegram_token_file"`
	DashboardAddress      string `json:"dashboard_address"`
}

func Load() (Settings, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Settings{}, fmt.Errorf("find user home directory: %w", err)
	}
	configFile := strings.TrimSpace(os.Getenv(EnvConfigFile))
	if configFile == "" {
		configFile = filepath.Join(home, ".config", "tren", "config.json")
	}
	configFile, err = expandPath(configFile, home, "")
	if err != nil {
		return Settings{}, fmt.Errorf("resolve config file: %w", err)
	}

	result := Settings{ConfigFile: configFile}
	contents, readErr := os.ReadFile(configFile)
	switch {
	case readErr == nil:
		result.ConfigFileFound = true
		decoder := json.NewDecoder(strings.NewReader(string(contents)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&result); err != nil {
			return Settings{}, fmt.Errorf("read config %s: %w", configFile, err)
		}
		if err := ensureJSONEnd(decoder); err != nil {
			return Settings{}, fmt.Errorf("read config %s: %w", configFile, err)
		}
	case errors.Is(readErr, os.ErrNotExist):
		// Environment-only configuration remains supported.
	default:
		return Settings{}, fmt.Errorf("read config %s: %w", configFile, readErr)
	}

	configDirectory := filepath.Dir(configFile)
	result.SpreadsheetID = chooseEnvironment(EnvSpreadsheetID, result.SpreadsheetID)
	result.GoogleCredentialsFile = chooseEnvironment(EnvGoogleCredentials, result.GoogleCredentialsFile)
	result.GoogleCredentialsJSON = strings.TrimSpace(os.Getenv(EnvGoogleCredentialsJSON))
	result.TelegramTokenFile = chooseEnvironment(EnvTelegramTokenFile, result.TelegramTokenFile)
	result.TelegramToken = strings.TrimSpace(os.Getenv(EnvTelegramToken))
	result.DashboardAddress = chooseEnvironment(EnvDashboardAddress, result.DashboardAddress)
	result.DashboardUser = chooseEnvironment(EnvDashboardUser, "tren")
	result.DashboardPassword = strings.TrimSpace(os.Getenv(EnvDashboardPassword))
	result.WebhookSecret = strings.TrimSpace(os.Getenv(EnvWebhookSecret))
	result.PublicURL = strings.TrimRight(strings.TrimSpace(os.Getenv(EnvPublicURL)), "/")
	result.Port = strings.TrimSpace(os.Getenv(EnvPort))
	if result.PublicURL == "" {
		if domain := strings.TrimSpace(os.Getenv(EnvRailwayPublicDomain)); domain != "" {
			result.PublicURL = "https://" + domain
		}
	}

	if result.GoogleCredentialsFile == "" {
		result.GoogleCredentialsFile = filepath.Join(home, ".config", "tren", "google-service-account.json")
	}
	if result.TelegramTokenFile == "" {
		result.TelegramTokenFile = filepath.Join(home, ".config", "tren", "telegram-token")
	}
	if result.DashboardAddress == "" {
		result.DashboardAddress = "127.0.0.1:8787"
	}
	result.GoogleCredentialsFile, err = expandPath(result.GoogleCredentialsFile, home, configDirectory)
	if err != nil {
		return Settings{}, fmt.Errorf("resolve Google credentials file: %w", err)
	}
	result.TelegramTokenFile, err = expandPath(result.TelegramTokenFile, home, configDirectory)
	if err != nil {
		return Settings{}, fmt.Errorf("resolve Telegram token file: %w", err)
	}
	return result, nil
}

func (settings Settings) ValidateSheets() error {
	if strings.TrimSpace(settings.SpreadsheetID) == "" {
		return fmt.Errorf("spreadsheet_id is not configured in %s or %s", settings.ConfigFile, EnvSpreadsheetID)
	}
	if strings.TrimSpace(settings.GoogleCredentialsFile) == "" && strings.TrimSpace(settings.GoogleCredentialsJSON) == "" {
		return fmt.Errorf("Google credentials are not configured")
	}
	return nil
}

func (settings Settings) ValidateBot() error {
	if err := settings.ValidateSheets(); err != nil {
		return err
	}
	if strings.TrimSpace(settings.TelegramTokenFile) == "" && strings.TrimSpace(settings.TelegramToken) == "" {
		return fmt.Errorf("Telegram token is not configured")
	}
	return nil
}

func (settings Settings) ValidateServe() error {
	if err := settings.ValidateBot(); err != nil {
		return err
	}
	if strings.TrimSpace(settings.WebhookSecret) == "" {
		return fmt.Errorf("%s is required for hosted webhook mode", EnvWebhookSecret)
	}
	if !validWebhookSecret(settings.WebhookSecret) {
		return fmt.Errorf("%s must be 1-256 characters using only letters, numbers, underscore, or hyphen", EnvWebhookSecret)
	}
	if strings.TrimSpace(settings.DashboardPassword) == "" {
		return fmt.Errorf("%s is required for the hosted dashboard", EnvDashboardPassword)
	}
	if settings.PublicURL != "" && !strings.HasPrefix(settings.PublicURL, "https://") {
		return fmt.Errorf("%s must use https://", EnvPublicURL)
	}
	return nil
}

func validWebhookSecret(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 1 || len(value) > 256 {
		return false
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '_' || character == '-' {
			continue
		}
		return false
	}
	return true
}

func (settings Settings) GoogleCredentialsSource() string {
	if value := strings.TrimSpace(settings.GoogleCredentialsJSON); value != "" {
		return value
	}
	return strings.TrimSpace(settings.GoogleCredentialsFile)
}

func (settings Settings) TelegramTokenValue() (string, error) {
	if value := strings.TrimSpace(settings.TelegramToken); value != "" {
		return value, nil
	}
	contents, err := os.ReadFile(settings.TelegramTokenFile)
	if err != nil {
		return "", fmt.Errorf("read Telegram token file: %w", err)
	}
	value := strings.TrimSpace(string(contents))
	if value == "" {
		return "", fmt.Errorf("Telegram bot token is empty")
	}
	return value, nil
}

func (settings Settings) ServeAddress() string {
	if settings.Port != "" {
		return ":" + settings.Port
	}
	if address := strings.TrimSpace(settings.DashboardAddress); address != "" {
		return address
	}
	return "127.0.0.1:8787"
}

func Save(settings Settings, overwrite bool) error {
	if strings.TrimSpace(settings.ConfigFile) == "" {
		return fmt.Errorf("config file path is required")
	}
	if err := settings.ValidateBot(); err != nil {
		return err
	}
	if !overwrite {
		if _, err := os.Stat(settings.ConfigFile); err == nil {
			return fmt.Errorf("config file %s already exists", settings.ConfigFile)
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect config file %s: %w", settings.ConfigFile, err)
		}
	}

	directory := filepath.Dir(settings.ConfigFile)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("find user home directory: %w", err)
	}
	googleCredentialsFile, err := expandPath(settings.GoogleCredentialsFile, home, directory)
	if err != nil {
		return fmt.Errorf("resolve Google credentials file: %w", err)
	}
	telegramTokenFile, err := expandPath(settings.TelegramTokenFile, home, directory)
	if err != nil {
		return fmt.Errorf("resolve Telegram token file: %w", err)
	}
	document := fileSettings{
		SpreadsheetID:         strings.TrimSpace(settings.SpreadsheetID),
		GoogleCredentialsFile: portablePath(googleCredentialsFile, directory),
		TelegramTokenFile:     portablePath(telegramTokenFile, directory),
		DashboardAddress:      strings.TrimSpace(settings.DashboardAddress),
	}
	contents, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	contents = append(contents, '\n')
	temporary, err := os.CreateTemp(directory, ".config-*.json")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return fmt.Errorf("protect temporary config: %w", err)
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return fmt.Errorf("write temporary config: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return fmt.Errorf("sync temporary config: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary config: %w", err)
	}
	if err := os.Rename(temporaryName, settings.ConfigFile); err != nil {
		// Unix replaces the destination atomically. Windows does not, so retain
		// cross-platform overwrite support with a narrow fallback.
		if !overwrite {
			return fmt.Errorf("install config: %w", err)
		}
		if removeErr := os.Remove(settings.ConfigFile); removeErr != nil {
			return fmt.Errorf("replace config: %w (after rename failed: %v)", removeErr, err)
		}
		if renameErr := os.Rename(temporaryName, settings.ConfigFile); renameErr != nil {
			return fmt.Errorf("install replacement config: %w", renameErr)
		}
	}
	return nil
}

func chooseEnvironment(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return strings.TrimSpace(fallback)
}

func expandPath(value, home, relativeTo string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if value == "~" {
		value = home
	} else if strings.HasPrefix(value, "~/") || strings.HasPrefix(value, `~\`) {
		value = filepath.Join(home, value[2:])
	} else if strings.HasPrefix(value, "~") {
		return "", fmt.Errorf("unsupported home path %q", value)
	}
	if !filepath.IsAbs(value) && relativeTo != "" {
		value = filepath.Join(relativeTo, value)
	}
	return filepath.Clean(value), nil
}

func portablePath(value, directory string) string {
	value = filepath.Clean(strings.TrimSpace(value))
	relative, err := filepath.Rel(directory, value)
	if err == nil && relative != "." && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return relative
	}
	return value
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("config contains more than one JSON value")
}

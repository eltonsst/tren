package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadReadsConfigAndResolvesPathsRelativeToIt(t *testing.T) {
	clearTrenEnvironment(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	configFile := filepath.Join(home, "custom", "config.json")
	if err := os.MkdirAll(filepath.Dir(configFile), 0o700); err != nil {
		t.Fatal(err)
	}
	contents := `{
  "spreadsheet_id": "sheet-from-file",
  "google_credentials_file": "credentials.json",
  "telegram_token_file": "~/secrets/telegram-token",
  "dashboard_address": "127.0.0.1:9999"
}`
	if err := os.WriteFile(configFile, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfigFile, configFile)

	settings, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if !settings.ConfigFileFound || settings.SpreadsheetID != "sheet-from-file" || settings.DashboardAddress != "127.0.0.1:9999" {
		t.Fatalf("settings = %#v", settings)
	}
	if settings.GoogleCredentialsFile != filepath.Join(home, "custom", "credentials.json") {
		t.Fatalf("credentials = %q", settings.GoogleCredentialsFile)
	}
	if settings.TelegramTokenFile != filepath.Join(home, "secrets", "telegram-token") {
		t.Fatalf("token = %q", settings.TelegramTokenFile)
	}
}

func TestLoadEnvironmentOverridesConfig(t *testing.T) {
	clearTrenEnvironment(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	configFile := filepath.Join(home, "config.json")
	if err := os.WriteFile(configFile, []byte(`{"spreadsheet_id":"file-sheet"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfigFile, configFile)
	t.Setenv(EnvSpreadsheetID, "environment-sheet")
	t.Setenv(EnvDashboardAddress, "localhost:9000")
	t.Setenv(EnvGoogleCredentialsJSON, `{"type":"service_account"}`)
	t.Setenv(EnvTelegramToken, "123456:abcdefghijklmnopqrstuvwxyz")
	t.Setenv(EnvDashboardPassword, "dashboard-secret")
	t.Setenv(EnvWebhookSecret, "webhook-secret")
	t.Setenv(EnvRailwayPublicDomain, "tren.up.railway.app")
	t.Setenv(EnvPort, "8080")

	settings, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.SpreadsheetID != "environment-sheet" || settings.DashboardAddress != "localhost:9000" {
		t.Fatalf("settings = %#v", settings)
	}
	if settings.GoogleCredentialsSource() != `{"type":"service_account"}` || settings.PublicURL != "https://tren.up.railway.app" || settings.ServeAddress() != ":8080" {
		t.Fatalf("hosted settings = %#v", settings)
	}
	if err := settings.ValidateServe(); err != nil {
		t.Fatalf("ValidateServe() error = %v", err)
	}
}

func TestLoadWithoutConfigPreservesLegacyDefaults(t *testing.T) {
	clearTrenEnvironment(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(EnvConfigFile, filepath.Join(home, "missing.json"))
	t.Setenv(EnvSpreadsheetID, "sheet")

	settings, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if settings.ConfigFileFound || settings.GoogleCredentialsFile != filepath.Join(home, ".config", "tren", "google-service-account.json") || settings.TelegramTokenFile != filepath.Join(home, ".config", "tren", "telegram-token") {
		t.Fatalf("settings = %#v", settings)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	clearTrenEnvironment(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	configFile := filepath.Join(home, "config.json")
	if err := os.WriteFile(configFile, []byte(`{"spreadsheet_id":"sheet","secret":"oops"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvConfigFile, configFile)
	if _, err := Load(); err == nil {
		t.Fatal("Load did not reject an unknown field")
	}
}

func TestValidateSheetsExplainsBothConfigurationOptions(t *testing.T) {
	settings := Settings{ConfigFile: "/tmp/tren/config.json"}
	if err := settings.ValidateSheets(); err == nil {
		t.Fatal("ValidateSheets accepted an empty spreadsheet ID")
	}
}

func TestValidateServeRejectsTelegramIncompatibleWebhookSecret(t *testing.T) {
	settings := Settings{
		SpreadsheetID: "sheet", GoogleCredentialsJSON: `{}`,
		TelegramToken: "token", WebhookSecret: "contains spaces",
		DashboardPassword: "dashboard-secret",
	}
	if err := settings.ValidateServe(); err == nil || !strings.Contains(err.Error(), EnvWebhookSecret) {
		t.Fatalf("ValidateServe() error = %v", err)
	}
}

func TestSaveWritesPrivatePortableConfigAndLoadsItAgain(t *testing.T) {
	clearTrenEnvironment(t)
	home := t.TempDir()
	t.Setenv("HOME", home)
	directory := filepath.Join(home, ".config", "tren")
	settings := Settings{
		SpreadsheetID: "sheet", ConfigFile: filepath.Join(directory, "config.json"),
		GoogleCredentialsFile: filepath.Join(directory, "google-service-account.json"),
		TelegramTokenFile:     filepath.Join(directory, "telegram-token"), DashboardAddress: "127.0.0.1:8787",
	}
	if err := Save(settings, false); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(settings.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}
	contents, err := os.ReadFile(settings.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(contents, &document); err != nil {
		t.Fatal(err)
	}
	if document["google_credentials_file"] != "google-service-account.json" || document["telegram_token_file"] != "telegram-token" {
		t.Fatalf("document = %#v", document)
	}
	t.Setenv(EnvConfigFile, settings.ConfigFile)
	loaded, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SpreadsheetID != "sheet" || loaded.GoogleCredentialsFile != settings.GoogleCredentialsFile || loaded.TelegramTokenFile != settings.TelegramTokenFile {
		t.Fatalf("loaded = %#v", loaded)
	}
}

func TestSaveRefusesExistingConfigWithoutOverwrite(t *testing.T) {
	directory := t.TempDir()
	settings := Settings{
		SpreadsheetID: "sheet", ConfigFile: filepath.Join(directory, "config.json"),
		GoogleCredentialsFile: filepath.Join(directory, "credentials.json"),
		TelegramTokenFile:     filepath.Join(directory, "token"), DashboardAddress: "127.0.0.1:8787",
	}
	if err := Save(settings, false); err != nil {
		t.Fatal(err)
	}
	if err := Save(settings, false); err == nil {
		t.Fatal("Save overwrote an existing config")
	}
}

func TestSaveReplacesExistingConfigWithConfirmation(t *testing.T) {
	directory := t.TempDir()
	settings := Settings{
		SpreadsheetID: "first", ConfigFile: filepath.Join(directory, "config.json"),
		GoogleCredentialsFile: filepath.Join(directory, "credentials.json"),
		TelegramTokenFile:     filepath.Join(directory, "token"), DashboardAddress: "127.0.0.1:8787",
	}
	if err := Save(settings, false); err != nil {
		t.Fatal(err)
	}
	settings.SpreadsheetID = "second"
	if err := Save(settings, true); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(settings.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), `"spreadsheet_id": "second"`) {
		t.Fatalf("config = %q", contents)
	}
}

func clearTrenEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range []string{
		EnvConfigFile, EnvSpreadsheetID, EnvGoogleCredentials,
		EnvGoogleCredentialsJSON, EnvTelegramTokenFile, EnvTelegramToken,
		EnvDashboardAddress, EnvDashboardUser, EnvDashboardPassword,
		EnvWebhookSecret, EnvPublicURL, EnvPort, EnvRailwayPublicDomain,
	} {
		t.Setenv(name, "")
	}
}

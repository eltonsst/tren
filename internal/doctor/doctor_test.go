package doctor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/eltonsst/tren/internal/config"
)

func TestLocalAcceptsCompleteConfiguration(t *testing.T) {
	directory := t.TempDir()
	credentials := filepath.Join(directory, "credentials.json")
	token := filepath.Join(directory, "telegram-token")
	if err := os.WriteFile(credentials, []byte(`{
  "type":"service_account",
  "client_email":"tren@example.test",
  "private_key":"private",
  "private_key_id":"key-id"
}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(token, []byte("123456789:abcdefghijklmnopqrstuvwxyz_ABCDE"), 0o600); err != nil {
		t.Fatal(err)
	}
	results := Local(config.Settings{
		SpreadsheetID: "sheet", GoogleCredentialsFile: credentials,
		TelegramTokenFile: token, DashboardAddress: "127.0.0.1:8787",
		ConfigFile: filepath.Join(directory, "config.json"), ConfigFileFound: true,
	})
	if HasFailures(results) {
		t.Fatalf("results = %#v", results)
	}
	for _, result := range results {
		if result.Status != OK {
			t.Fatalf("result = %#v", result)
		}
	}
}

func TestLocalReportsAllMissingRequirements(t *testing.T) {
	directory := t.TempDir()
	results := Local(config.Settings{
		GoogleCredentialsFile: filepath.Join(directory, "missing-google.json"),
		TelegramTokenFile:     filepath.Join(directory, "missing-token"),
		DashboardAddress:      "not-an-address", ConfigFile: filepath.Join(directory, "missing-config.json"),
	})
	if !HasFailures(results) {
		t.Fatalf("results = %#v", results)
	}
	failed := 0
	warnings := 0
	for _, result := range results {
		if result.Status == Failed {
			failed++
		}
		if result.Status == Warning {
			warnings++
		}
	}
	if failed != 4 || warnings != 1 {
		t.Fatalf("failed = %d, warnings = %d, results = %#v", failed, warnings, results)
	}
}

func TestLocalWarnsAboutPrivateFilePermissions(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("Unix permission bits are not portable to Windows")
	}
	directory := t.TempDir()
	credentials := filepath.Join(directory, "credentials.json")
	token := filepath.Join(directory, "telegram-token")
	if err := os.WriteFile(credentials, []byte(`{"type":"service_account","client_email":"tren@example.test","private_key":"private","private_key_id":"key-id"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(token, []byte("123456789:abcdefghijklmnopqrstuvwxyz_ABCDE"), 0o644); err != nil {
		t.Fatal(err)
	}
	results := Local(config.Settings{
		SpreadsheetID: "sheet", GoogleCredentialsFile: credentials,
		TelegramTokenFile: token, DashboardAddress: "127.0.0.1:8787",
		ConfigFile: filepath.Join(directory, "config.json"), ConfigFileFound: true,
	})
	warnings := 0
	for _, result := range results {
		if result.Status == Warning {
			warnings++
		}
	}
	if warnings != 2 || HasFailures(results) {
		t.Fatalf("results = %#v", results)
	}
}

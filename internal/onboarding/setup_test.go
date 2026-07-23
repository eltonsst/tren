package onboarding

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eltonsst/tren/internal/config"
)

func TestSetupCreatesConfigAndExplainsNextStep(t *testing.T) {
	directory := t.TempDir()
	settings := config.Settings{
		ConfigFile:            filepath.Join(directory, "config.json"),
		GoogleCredentialsFile: filepath.Join(directory, "google-service-account.json"),
		TelegramTokenFile:     filepath.Join(directory, "telegram-token"),
		DashboardAddress:      "127.0.0.1:8787",
	}
	input := strings.NewReader("sheet-123\n\n\n\n")
	var output bytes.Buffer
	written, err := Setup(settings, input, &output)
	if err != nil || !written {
		t.Fatalf("written = %v, err = %v, output = %q", written, err, output.String())
	}
	contents, err := os.ReadFile(settings.ConfigFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), `"spreadsheet_id": "sheet-123"`) || !strings.Contains(output.String(), "Run `tren doctor` next") {
		t.Fatalf("config = %q, output = %q", contents, output.String())
	}
}

func TestSetupDoesNotOverwriteWithoutConfirmation(t *testing.T) {
	directory := t.TempDir()
	configFile := filepath.Join(directory, "config.json")
	original := []byte("original")
	if err := os.WriteFile(configFile, original, 0o600); err != nil {
		t.Fatal(err)
	}
	settings := config.Settings{
		ConfigFile: configFile, ConfigFileFound: true, SpreadsheetID: "sheet",
		GoogleCredentialsFile: filepath.Join(directory, "credentials.json"),
		TelegramTokenFile:     filepath.Join(directory, "token"), DashboardAddress: "127.0.0.1:8787",
	}
	var output bytes.Buffer
	written, err := Setup(settings, strings.NewReader("\n\n\n\nn\n"), &output)
	if err != nil || written {
		t.Fatalf("written = %v, err = %v", written, err)
	}
	contents, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != string(original) || !strings.Contains(output.String(), "not changed") {
		t.Fatalf("config = %q, output = %q", contents, output.String())
	}
}

func TestSetupRejectsMissingSpreadsheetID(t *testing.T) {
	directory := t.TempDir()
	settings := config.Settings{
		ConfigFile:            filepath.Join(directory, "config.json"),
		GoogleCredentialsFile: filepath.Join(directory, "credentials.json"),
		TelegramTokenFile:     filepath.Join(directory, "token"), DashboardAddress: "127.0.0.1:8787",
	}
	var output bytes.Buffer
	written, err := Setup(settings, strings.NewReader("\n"), &output)
	if err == nil || written {
		t.Fatalf("written = %v, err = %v", written, err)
	}
}

package doctor

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/eltonsst/tren/internal/config"
)

type Status string

const (
	OK      Status = "ok"
	Warning Status = "warning"
	Failed  Status = "failed"
	Skipped Status = "skipped"
)

type Result struct {
	Name   string
	Status Status
	Detail string
}

func Local(settings config.Settings) []Result {
	results := make([]Result, 0, 5)
	if settings.ConfigFileFound {
		results = append(results, Result{Name: "Configuration", Status: OK, Detail: settings.ConfigFile})
	} else {
		results = append(results, Result{
			Name: "Configuration", Status: Warning,
			Detail: fmt.Sprintf("%s is absent; using environment variables and defaults", settings.ConfigFile),
		})
	}
	if strings.TrimSpace(settings.SpreadsheetID) == "" {
		results = append(results, Result{Name: "Spreadsheet ID", Status: Failed, Detail: "not configured"})
	} else {
		results = append(results, Result{Name: "Spreadsheet ID", Status: OK, Detail: "configured"})
	}
	if strings.TrimSpace(settings.GoogleCredentialsJSON) != "" {
		results = append(results, inspectServiceAccountContents([]byte(settings.GoogleCredentialsJSON), 0, false))
	} else {
		results = append(results, inspectServiceAccount(settings.GoogleCredentialsFile))
	}
	if strings.TrimSpace(settings.TelegramToken) != "" {
		results = append(results, inspectTelegramTokenValue(settings.TelegramToken, 0, false))
	} else {
		results = append(results, inspectTelegramToken(settings.TelegramTokenFile))
	}
	if _, err := net.ResolveTCPAddr("tcp", settings.DashboardAddress); err != nil {
		results = append(results, Result{Name: "Dashboard address", Status: Failed, Detail: err.Error()})
	} else {
		results = append(results, Result{Name: "Dashboard address", Status: OK, Detail: settings.DashboardAddress})
	}
	return results
}

func HasFailures(results []Result) bool {
	for _, result := range results {
		if result.Status == Failed {
			return true
		}
	}
	return false
}

func inspectServiceAccount(path string) Result {
	if strings.TrimSpace(path) == "" {
		return Result{Name: "Google credentials", Status: Failed, Detail: "file path is not configured"}
	}
	contents, mode, err := readPrivateFile(path)
	if err != nil {
		return Result{Name: "Google credentials", Status: Failed, Detail: err.Error()}
	}
	return inspectServiceAccountContents(contents, mode, true)
}

func inspectServiceAccountContents(contents []byte, mode os.FileMode, checkMode bool) Result {
	var credentials struct {
		Type         string `json:"type"`
		ClientEmail  string `json:"client_email"`
		PrivateKey   string `json:"private_key"`
		PrivateKeyID string `json:"private_key_id"`
	}
	if err := json.Unmarshal(contents, &credentials); err != nil {
		return Result{Name: "Google credentials", Status: Failed, Detail: "invalid service-account JSON"}
	}
	if credentials.Type != "service_account" || strings.TrimSpace(credentials.ClientEmail) == "" || strings.TrimSpace(credentials.PrivateKey) == "" || strings.TrimSpace(credentials.PrivateKeyID) == "" {
		return Result{Name: "Google credentials", Status: Failed, Detail: "JSON is not a complete service account"}
	}
	detail := credentials.ClientEmail
	if checkMode && mode&0o077 != 0 {
		return Result{Name: "Google credentials", Status: Warning, Detail: detail + "; file is readable by other local users"}
	}
	return Result{Name: "Google credentials", Status: OK, Detail: detail}
}

func inspectTelegramToken(path string) Result {
	if strings.TrimSpace(path) == "" {
		return Result{Name: "Telegram token", Status: Failed, Detail: "file path is not configured"}
	}
	contents, mode, err := readPrivateFile(path)
	if err != nil {
		return Result{Name: "Telegram token", Status: Failed, Detail: err.Error()}
	}
	return inspectTelegramTokenValue(string(contents), mode, true)
}

func inspectTelegramTokenValue(value string, mode os.FileMode, checkMode bool) Result {
	token := strings.TrimSpace(value)
	parts := strings.Split(token, ":")
	if len(parts) != 2 || len(parts[0]) < 6 || len(parts[1]) < 20 {
		return Result{Name: "Telegram token", Status: Failed, Detail: "file does not contain a plausible bot token"}
	}
	if checkMode && mode&0o077 != 0 {
		return Result{Name: "Telegram token", Status: Warning, Detail: "configured; file is readable by other local users"}
	}
	return Result{Name: "Telegram token", Status: OK, Detail: "configured"}
}

func readPrivateFile(path string) ([]byte, os.FileMode, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", path, err)
	}
	if !info.Mode().IsRegular() {
		return nil, 0, fmt.Errorf("%s is not a regular file", path)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("%s: %w", path, err)
	}
	return contents, info.Mode().Perm(), nil
}

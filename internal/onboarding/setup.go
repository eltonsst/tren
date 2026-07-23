package onboarding

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/eltonsst/tren/internal/config"
)

func Setup(settings config.Settings, input io.Reader, output io.Writer) (bool, error) {
	reader := bufio.NewReader(input)
	fmt.Fprintf(output, "Tren setup\nConfig file: %s\nPress Enter to keep an existing/default value.\n\n", settings.ConfigFile)

	spreadsheetID, err := prompt(reader, output, "Google spreadsheet ID", settings.SpreadsheetID)
	if err != nil {
		return false, err
	}
	if spreadsheetID == "" {
		return false, fmt.Errorf("Google spreadsheet ID is required")
	}
	credentialsFile, err := prompt(reader, output, "Google service-account JSON", settings.GoogleCredentialsFile)
	if err != nil {
		return false, err
	}
	tokenFile, err := prompt(reader, output, "Telegram token file", settings.TelegramTokenFile)
	if err != nil {
		return false, err
	}
	dashboardAddress, err := prompt(reader, output, "Dashboard address", settings.DashboardAddress)
	if err != nil {
		return false, err
	}

	overwrite := false
	if settings.ConfigFileFound {
		answer, err := prompt(reader, output, "Overwrite the existing config? (y/N)", "N")
		if err != nil {
			return false, err
		}
		if !strings.EqualFold(answer, "y") && !strings.EqualFold(answer, "yes") {
			fmt.Fprintln(output, "Setup canceled; the existing config was not changed.")
			return false, nil
		}
		overwrite = true
	}

	settings.SpreadsheetID = spreadsheetID
	settings.GoogleCredentialsFile = credentialsFile
	settings.TelegramTokenFile = tokenFile
	settings.DashboardAddress = dashboardAddress
	if err := config.Save(settings, overwrite); err != nil {
		return false, err
	}
	fmt.Fprintf(output, "\nSaved %s with private file permissions.\nRun `tren doctor` next.\n", settings.ConfigFile)
	return true, nil
}

func prompt(reader *bufio.Reader, output io.Writer, label, current string) (string, error) {
	current = strings.TrimSpace(current)
	if current == "" {
		fmt.Fprintf(output, "%s: ", label)
	} else {
		fmt.Fprintf(output, "%s [%s]: ", label, current)
	}
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("read %s: %w", label, err)
	}
	if err == io.EOF && line == "" {
		return "", fmt.Errorf("read %s: %w", label, io.EOF)
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return current, nil
	}
	return value, nil
}

# Tren onboarding

Tren is self-hosted and requires no AI model, API key, subscription, or application database. Each installation uses its own Telegram bot and Google Sheet.

## Prerequisites

- A supported container host such as Railway, or a computer for local-only operation
- A Google account and Google Cloud project
- A Telegram account
- Go for source builds; prebuilt binaries are planned for the public release

## 1. Create the Sheet

Import `tren-data-contract.xlsx` into Google Sheets and begin with its `Start Here` tab. The included A–D rotation is example starter data: edit it in `Exercises` and `Program` before training if needed. Keep the required tab names and header names unchanged; columns may be reordered.

Copy the spreadsheet ID from its URL:

```text
https://docs.google.com/spreadsheets/d/SPREADSHEET_ID/edit
```

## 2. Create Google credentials

Enable the Google Sheets API in a Google Cloud project, create a service account, and download its JSON key. Store it outside the Tren repository.

Share only the Tren Google Sheet with the service account's `client_email` address and grant Editor access. The bot needs write access; statistics and diagnostics use read-only API scopes.

## 3. Create the Telegram bot

Create a bot with `@BotFather` and copy its token into a plain-text file. Do not commit or share the token.

Start Tren once, send `/whoami`, and place the returned numeric user ID in the Sheet under `Config` → `authorized_telegram_user_id`. This keeps each self-hosted bot private to its owner.

## 4. Choose hosted or local operation

For an always-available bot and remote dashboard, follow [`hosting.md`](hosting.md). The hosted service reads the Google and Telegram secrets directly from environment variables, so secret files are not included in the deployment.

Run the interactive setup and accept the suggested secret-file locations, or enter your own:

```bash
tren setup
```

This creates `~/.config/tren/config.json` with private file permissions. To configure it manually instead, use `config.example.json`:

```json
{
  "spreadsheet_id": "your-google-spreadsheet-id",
  "google_credentials_file": "google-service-account.json",
  "telegram_token_file": "telegram-token",
  "dashboard_address": "127.0.0.1:8787"
}
```

Relative file paths are resolved from the directory containing `config.json`; paths beginning with `~/` are also supported. Keep the credential and token files readable only by your local user:

```bash
chmod 600 ~/.config/tren/google-service-account.json
chmod 600 ~/.config/tren/telegram-token
```

Set `TREN_CONFIG` to use a different config-file location. `TREN_SHEET_ID`, `TREN_GOOGLE_CREDENTIALS`, `TREN_TELEGRAM_TOKEN_FILE`, and `TREN_DASHBOARD_ADDR` override local config-file values. Hosted deployments instead use the variables documented in [`hosting.md`](hosting.md) and [`.env.example`](../.env.example).

## 5. Diagnose and run locally

```bash
tren doctor
tren bot
```

`tren doctor` validates the local configuration, secret-file structure and permissions, workbook schema, active program, and Telegram bot identity. It reads the Sheet but never modifies it and never sends a Telegram message.

The bot uses long polling. Keep the Tren process and computer running while responses are needed; Telegram can retain pending updates temporarily while it is offline.

Hosted mode uses `tren serve` and Telegram webhooks; it does not require a computer to remain running.

## Security notes

- Never place the service-account JSON, Telegram token, or a populated personal config file in the repository.
- Bind the dashboard to `127.0.0.1` unless network access is intentionally configured and protected.
- Hosted dashboards require `TREN_DASHBOARD_PASSWORD`; webhook requests require `TREN_WEBHOOK_SECRET`.
- Use an independent bot, Sheet, and service account for each self-hosted user.
- Run `tren doctor` after changing credentials or copying a new workbook.

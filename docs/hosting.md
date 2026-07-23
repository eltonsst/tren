# Hosting Tren

Tren runs as one portable HTTP service. It receives Telegram webhook updates, reads and writes the owner's Google Sheet, and serves the statistics dashboard. The container keeps no durable local data.

## Required environment variables

| Variable | Purpose |
| --- | --- |
| `TREN_SHEET_ID` | Google spreadsheet ID |
| `TREN_GOOGLE_CREDENTIALS_JSON` | Complete service-account JSON document |
| `TREN_TELEGRAM_TOKEN` | Token issued by BotFather |
| `TREN_WEBHOOK_SECRET` | Random value used to verify Telegram webhook requests |
| `TREN_DASHBOARD_PASSWORD` | Password protecting the browser dashboard |

`TREN_DASHBOARD_USER` defaults to `tren`. Railway supplies `PORT` and `RAILWAY_PUBLIC_DOMAIN`; on another provider, set `PORT` and `TREN_PUBLIC_URL` explicitly. `TREN_PUBLIC_URL` must be the public HTTPS origin without a trailing slash.

Generate independent secrets rather than reusing the Telegram token or dashboard password:

```bash
openssl rand -hex 32
```

Never place the completed `.env` file or service-account JSON in the repository.

## Railway Free

1. Create a Railway account and an empty project. A payment method is not required for the Free plan.
2. Create an empty service.
3. From the Tren repository, authenticate and deploy the local directory:

   ```bash
   railway login
   railway link
   railway up
   ```

4. In the service Variables page, add the five required variables above. Optionally set `TREN_DASHBOARD_USER`.
5. In Networking, generate a Railway domain.
6. Enable Serverless in the service deployment settings so Tren sleeps while inactive.
7. Review current pricing and cost controls before deploying. Railway's Free plan has included monthly usage, while custom Compute hard limits may have a platform-enforced minimum. Do not upgrade to a paid plan unless that cost is intentional.
8. Redeploy once after the domain exists. Tren derives its public URL from `RAILWAY_PUBLIC_DOMAIN` and registers `/telegram/webhook` with Telegram during startup.
9. Open the generated domain and enter the configured dashboard username and password.
10. Send `/current` to the Telegram bot and confirm that it responds.

The health endpoint is `GET /healthz`. It intentionally requires no password and reveals only `ok`. Every dashboard route uses HTTP Basic authentication. The webhook requires Telegram's `X-Telegram-Bot-Api-Secret-Token` header before parsing or accessing Google Sheets.

Railway uses the health endpoint only while activating a deployment, not as continuous monitoring, so it does not keep Serverless awake. Tren makes no periodic outbound requests. After the final Telegram or dashboard operation, it becomes eligible to sleep once Railway's inactivity window passes.

Railway pricing and limits can change. Treat the provider documentation and the workspace Usage page as authoritative rather than relying on values copied into this repository.

## Other providers

Build the included Dockerfile and supply the same environment variables. The platform must provide:

- an HTTPS public URL;
- a TCP port through `PORT`;
- outbound HTTPS access to Telegram and Google APIs;
- enough request time for Google Sheets operations.

Set `TREN_PUBLIC_URL` to the public origin. No disk or database is required. Moving providers does not move training data because Google Sheets remains the source of truth.

## Local hosted-mode test

Keep `TREN_PUBLIC_URL` unset so a local test does not replace the active Telegram webhook:

```bash
TREN_WEBHOOK_SECRET=local-test-secret \
TREN_DASHBOARD_PASSWORD=local-test-password \
go run ./cmd/tren serve
```

This starts the combined service using the normal local config and logs that webhook registration was skipped. Open `http://127.0.0.1:8787` and authenticate as `tren`.

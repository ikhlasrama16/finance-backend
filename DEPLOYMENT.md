# Deployment readiness

The API reads configuration from environment variables and does not run migrations automatically.

Required variables:

```env
APP_ENV=production
PORT=8080
DATABASE_URL=postgresql://...
INGEST_API_KEY=...
OPENROUTER_API_KEY=...
OPENROUTER_MODEL=openrouter/free
CORS_ALLOWED_ORIGINS=https://app.example.com
```

`CORS_ALLOWED_ORIGINS` is a comma-separated list. In production, only listed origins receive CORS headers. Keep it empty when browser-origin access is not needed. Secrets must be supplied by the deployment environment and must not be committed.

## Migration workflow

1. Configure `DATABASE_URL` against the target PostgreSQL database.
2. Apply migrations in numeric order:

   ```text
   001_initial_schema.sql
   002_parser_support.sql
   003_seed_parser_categories.sql
   004_seed_seabank_account.sql
   005_add_legacy_transaction_id.sql
   006_seed_legacy_master_data.sql
   007_seed_reconciliation_categories.sql
   008_create_ai_reports.sql
   009_add_detached_raw_notification_status.sql
   ```

3. Start the application with the same `DATABASE_URL`.

Example:

```bash
psql "$DATABASE_URL" -f migrations/001_initial_schema.sql
psql "$DATABASE_URL" -f migrations/002_parser_support.sql
psql "$DATABASE_URL" -f migrations/003_seed_parser_categories.sql
psql "$DATABASE_URL" -f migrations/004_seed_seabank_account.sql
psql "$DATABASE_URL" -f migrations/005_add_legacy_transaction_id.sql
psql "$DATABASE_URL" -f migrations/006_seed_legacy_master_data.sql
psql "$DATABASE_URL" -f migrations/007_seed_reconciliation_categories.sql
psql "$DATABASE_URL" -f migrations/008_create_ai_reports.sql
psql "$DATABASE_URL" -f migrations/009_add_detached_raw_notification_status.sql
go run ./cmd/api
```

Never edit an already-applied migration. Add a new numbered migration for later schema changes.

The liveness endpoint is `GET /api/v1/health`; PostgreSQL readiness is checked by `GET /api/v1/ready`.

## AI report configuration

`POST /api/v1/reports/ai` always returns deterministic PostgreSQL statistics when aggregation succeeds. `OPENROUTER_API_KEY` is only needed to generate the optional natural-language `ai` content; without it or during an OpenRouter outage, that field reports `status: "unavailable"`.

Report dates use the `Asia/Jakarta` calendar and `transactions.occurred_at`. Daily covers the current local day. Weekly covers Monday through the current local day and compares the same weekdays one week earlier. Monthly covers the first day of the current month through the current local day and compares the same month-to-date dates in the previous month (clamped to its last day). Custom periods compare against an immediately preceding window of the same number of calendar days.

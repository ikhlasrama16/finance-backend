# Deployment readiness

The API reads configuration from environment variables and does not run migrations automatically.

Required variables:

```env
APP_ENV=production
PORT=8080
DATABASE_URL=postgresql://...
INGEST_API_KEY=...
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
   ```

3. Start the application with the same `DATABASE_URL`.

Example:

```bash
psql "$DATABASE_URL" -f migrations/001_initial_schema.sql
psql "$DATABASE_URL" -f migrations/002_parser_support.sql
psql "$DATABASE_URL" -f migrations/003_seed_parser_categories.sql
go run ./cmd/api
```

Never edit an already-applied migration. Add a new numbered migration for later schema changes.

The liveness endpoint is `GET /api/v1/health`; PostgreSQL readiness is checked by `GET /api/v1/ready`.

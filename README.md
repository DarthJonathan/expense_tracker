# Shared Expense Tracker (Frontend + Backend)

This repository is now split into:

- `frontend/`: SvelteKit PWA (offline-first UI + IndexedDB local state)
- `backend/`: Go API service for syncing group data to PostgreSQL

The frontend no longer uses Supabase client SDK directly. It syncs through the backend API (`/api/v1/sync`), and the backend writes to Postgres.
In deployment, frontend calls same-origin `/api/v1/*` and SvelteKit server proxies to backend (BFF pattern).

## Project structure

- [`/Users/nathan/Development/expense_tracker/frontend`](/Users/nathan/Development/expense_tracker/frontend)
- [`/Users/nathan/Development/expense_tracker/backend`](/Users/nathan/Development/expense_tracker/backend)
- [`/Users/nathan/Development/expense_tracker/backend/database/schema.sql`](/Users/nathan/Development/expense_tracker/backend/database/schema.sql)
- [`/Users/nathan/Development/expense_tracker/docker-compose.yml`](/Users/nathan/Development/expense_tracker/docker-compose.yml)

## Run with Docker (recommended)

```bash
cp .env.example .env
docker compose up --build
```

Services:

- Frontend: `http://localhost:3000`
- Backend API: `http://localhost:8080`
- Swagger UI: `http://localhost:8080/swagger/index.html`
- Postgres: `localhost:5432`

## Local development

### 1) Backend

```bash
cd backend
cp .env.example .env
go run .
```

### 2) Frontend

```bash
cd frontend
cp .env.example .env
npm install
npm run dev
```

Frontend env:

- `BACKEND_API_URL` (server-only; used by SvelteKit BFF proxy to reach backend, e.g. `http://backend:8080`)

Backend env:

- `DATABASE_URL`
- `PORT`
- `CORS_ALLOW_ORIGIN`
- `JWT_SECRET`
- `JWT_TOKEN_EXPIRY_HOURS`

## Apple Shortcut automation

Authenticated Apple Shortcuts can create a transaction with `POST /api/v1/entries/apple` (the
`/api/v1/entries/automation` alias accepts the same payload):

```json
{
  "createdAt": "2026-08-09T12:30:00+08:00",
  "accountType": "card",
  "merchant": "Example merchant",
  "amount": "12.34",
  "currency": "USD",
  "device": "iPhone"
}
```

`currency` is optional and defaults to `SGD`. Amounts are stored in their original currency and
converted to the user's base currency using the transaction date. The response includes
`baseAmount`, `baseCurrency`, `fxRate`, and `fxRateDate`.

## Notes

- Sync is still offline-first: local changes are saved immediately and pushed when online.
- For shared family/group workflows, records are scoped by `activeGroupId` and merged by `updatedAt`.
- IDs should be UUIDs to match PostgreSQL UUID columns.

## Private statement ingestion

PDF statements are decoded in the browser with the bundled PDF.js worker. The PDF and extracted page text are checkpointed only in IndexedDB and are never accepted by the statement API. After parsing, the client sends normalized statement controls and transaction rows to the backend for durable review.

The server stores review state in `expense_statement_ingestions` and `expense_statement_ingestion_rows`. It suggests matches against existing transactions, recomputes reconciliation checks after edits, and confirms all reviewed rows in one transaction. Confirmed expenses retain ingestion provenance in `expense_entries.metadata`. Soft-deleting staging data does not delete confirmed expenses.

The migration is additive and idempotent. It creates the new staging tables and indexes without rewriting existing expense data. A PostgreSQL preservation test can be run with:

```bash
cd backend
MIGRATION_TEST_DATABASE_URL='postgres://...' go test ./database -run TestMigrateAddsStatementTablesWithoutChangingExistingExpenses
```

Text-layer PDFs are supported directly. Image-only statements require a deployment-provided on-device OCR worker through `globalThis.__spenditLocalPdfOcr`; there is intentionally no cloud OCR fallback.

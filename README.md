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
- Shared family/group records are scoped by the server-assigned group. The server is authoritative; concurrent local/server edits require a choice during sync.
- IDs should be UUIDs to match PostgreSQL UUID columns.

## Sync behavior

Sync uploads only records that differ from the device's durable acknowledgements, including soft deletions. Requests contain at most 100 records and 128 KiB of JSON, with accounts and categories sent before their dependent transactions. Accepted canonical records and acknowledgements are saved in IndexedDB so retries resume after a failed batch.

Downloads use UUID keyset pages of 100 records per collection. The client sends compact server-issued content versions for records it already has, and the backend returns only new or changed records plus the next page cursor. This avoids relying on device clocks for change detection. The backend still checks each page; this is not a database change-feed implementation. Sync calls are serialized, and edits made during a sync trigger another pass.

The server checks each edit against the server version last seen by that device, under a row lock. A conflict opens a comparison dialog with **Keep server version**, **Keep my local version**, and **Decide later**. Local choices are rechecked against the displayed server version before writing; another intervening server edit causes another conflict. Server choices replace the local snapshot regardless of device timestamps, while edits made after that snapshot remain pending. Deciding later preserves the local edit and leaves the conflict unresolved. New records and edits based on an unchanged server version sync automatically.

Server timestamps, group identity, base currency, FX calculations, and the merchant catalogue are canonical server fields. Sync does not use device clocks to select a winner. Deploy the backend before the frontend. The frontend requires protocol version 3; older clients must refresh before sending edits that would conflict with existing server records. Existing IndexedDB records and earlier sync checkpoints are preserved.

The optional PostgreSQL integration checks for conflict handling, concurrent writers, and group isolation run with `SYNC_TEST_DATABASE_URL='postgres://...' go test ./service -run TestSyncServerAuthorityPostgres` from `backend/`. They use a temporary test schema.

Failures are logged in the browser console and shown in a persistent banner with error details. The HTTP status and request ID link a failed request to frontend proxy and backend logs. Proxy connection failures return JSON with HTTP 502. To inspect Docker logs, use `docker compose logs --tail=100 frontend backend`.

A sync 403 means a submitted UUID belongs to an inaccessible group or personal category. The
error includes the collection and record ID; backend failure logs also include `sync_collection`,
`sync_record_id`, and counts of the submitted collections. Error responses also identify the
submitted UUID in `inaccessibleRecord`, without disclosing private record contents. The client
keeps cached personal categories owned by another user on the device, excludes them from that user's sync, and does not
mark their pending edits as uploaded. Server ownership checks still apply to every submitted record.

The server is authoritative when category sharing or ownership changes. Visible category conflicts
with different scope or ownership automatically take the server copy. If an uploaded category is
now inaccessible, sync removes that stale category from the local cache and retries the remaining
records in the rejected batch. Transactions and adjustments are preserved, and the server category
is never overwritten or recreated. A category that becomes visible again is downloaded normally.
Ordinary edit conflicts retain the existing choice dialog; generic 403 responses and inaccessible
records outside the submitted category batch still stop sync without clearing local records. The
client also understands the record-specific diagnostic text from earlier backends.

## Home analytics

The mobile home page and desktop dashboard include a 6/12-month spending chart with household,
personal, and category filters. Selecting a month updates its total, category breakdown, and insights.
The current month is marked as partial and compared with the same elapsed dates in the previous
month; completed months use full-month comparisons. Totals use the user's base currency, exclude
income and future/deleted transactions, and retain spending in archived categories. Foreign expenses
awaiting conversion are marked as pending rather than counted at a 1:1 exchange rate.

Spending insights run on-device using repeat coffee shop purchases, frequent merchants, category
increases, and monthly category targets. Suggestions need a minimum sample and show the recorded
amounts behind them. Savings are optional scenarios, and budget pacing is an estimate. This is
pattern-based analysis; it does not call an AI service or send transaction data to one.

Run `npm run test:analytics` from `frontend/` for analytics checks.

## Private statement ingestion

PDF statements are decoded in the browser with the bundled PDF.js worker. The PDF and extracted page text are checkpointed only in IndexedDB and are never accepted by the statement API. After parsing, the client sends normalized statement controls and transaction rows to the backend for durable review.

The server stores review state in `expense_statement_ingestions` and `expense_statement_ingestion_rows`. It suggests matches against existing transactions, recomputes reconciliation checks after edits, and confirms all reviewed rows in one transaction. Confirmed expenses retain ingestion provenance in `expense_entries.metadata`. Soft-deleting staging data does not delete confirmed expenses.

The migration is additive and idempotent. It creates the new staging tables and indexes without rewriting existing expense data. A PostgreSQL preservation test can be run with:

```bash
cd backend
MIGRATION_TEST_DATABASE_URL='postgres://...' go test ./database -run TestMigrateAddsStatementTablesWithoutChangingExistingExpenses
```

Text-layer PDFs are supported directly. Image-only statements require a deployment-provided on-device OCR worker through `globalThis.__spenditLocalPdfOcr`; there is intentionally no cloud OCR fallback.

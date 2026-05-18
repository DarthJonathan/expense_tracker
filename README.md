# Shared Expense Tracker (Frontend + Backend)

This repository is now split into:

- `frontend/`: SvelteKit PWA (offline-first UI + IndexedDB local state)
- `backend/`: Go API service for syncing group data to PostgreSQL

The frontend no longer uses Supabase client SDK directly. It syncs through the backend API (`/api/v1/sync`), and the backend writes to Postgres.

## Project structure

- [`/Users/nathan/Development/expense_tracker/frontend`](/Users/nathan/Development/expense_tracker/frontend)
- [`/Users/nathan/Development/expense_tracker/backend`](/Users/nathan/Development/expense_tracker/backend)
- [`/Users/nathan/Development/expense_tracker/backend/database/schema.sql`](/Users/nathan/Development/expense_tracker/backend/database/schema.sql)
- [`/Users/nathan/Development/expense_tracker/docker-compose.yml`](/Users/nathan/Development/expense_tracker/docker-compose.yml)

## Run with Docker (recommended)

```bash
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

- `VITE_API_BASE_URL` (default example: `http://localhost:8080`)
- `API_BASE_URL` (container/runtime config, preferred for Docker deploys)

Backend env:

- `DATABASE_URL`
- `PORT`
- `CORS_ALLOW_ORIGIN`
- `JWT_SECRET`
- `JWT_TOKEN_EXPIRY_HOURS`

## Notes

- Sync is still offline-first: local changes are saved immediately and pushed when online.
- For shared family/group workflows, records are scoped by `activeGroupId` and merged by `updatedAt`.
- IDs should be UUIDs to match PostgreSQL UUID columns.

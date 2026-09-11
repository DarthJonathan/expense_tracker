create extension if not exists pgcrypto;
create extension if not exists pg_trgm;
create extension if not exists unaccent;
create schema if not exists spendit;

create table if not exists spendit.expense_users (
	id uuid primary key default gen_random_uuid(),
	email text not null unique,
	password_hash text not null,
	display_name text not null,
	base_currency text not null default 'SGD',
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create table if not exists spendit.expense_groups (
	id uuid primary key default gen_random_uuid(),
	name text not null,
	invite_code text not null unique,
	created_by uuid,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

alter table spendit.expense_users
	add column if not exists group_id uuid references spendit.expense_groups(id) on delete set null;

create table if not exists spendit.expense_accounts (
	id uuid primary key default gen_random_uuid(),
	group_id uuid not null references spendit.expense_groups(id) on delete cascade,
	name text not null,
	type text not null check (type in ('cash', 'bank', 'card', 'wallet')),
	opening_balance integer not null default 0,
	fx_markup_percent numeric(6,3) not null default 3.5 check (fx_markup_percent >= 0 and fx_markup_percent <= 100),
	color text not null default '#4b5745',
	icon text not null default '🏦',
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create table if not exists spendit.expense_categories (
	id uuid primary key default gen_random_uuid(),
	group_id uuid not null references spendit.expense_groups(id) on delete cascade,
	name text not null,
	type text not null default 'expense' check (type in ('expense', 'income')),
	scope text not null default 'household' check (scope in ('household', 'user')),
	owner_user_id uuid references spendit.expense_users(id) on delete set null,
	color text not null default '#e7d24e',
	icon text not null default '🏷️',
	monthly_target integer not null default 0,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create table if not exists spendit.expense_entries (
	id uuid primary key default gen_random_uuid(),
	group_id uuid not null references spendit.expense_groups(id) on delete cascade,
	account_id uuid not null references spendit.expense_accounts(id) on delete restrict,
	category_id uuid not null references spendit.expense_categories(id) on delete restrict,
	type text not null check (type in ('expense', 'income')),
	amount integer not null check (amount >= 0),
	currency text not null default 'SGD',
	base_amount integer not null default 0,
	base_currency text not null default 'SGD',
	fx_rate numeric(20,10) not null default 1,
	fx_rate_date date not null default current_date,
	occurred_on date not null,
	merchant text not null,
	note text not null default '',
	metadata jsonb not null default '{}'::jsonb,
	created_by uuid,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create table if not exists spendit.expense_category_adjustments (
	id uuid primary key default gen_random_uuid(),
	group_id uuid not null references spendit.expense_groups(id) on delete cascade,
	category_id uuid not null references spendit.expense_categories(id) on delete cascade,
	amount integer not null,
	occurred_on date not null,
	note text not null default '',
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create table if not exists spendit.expense_merchants (
	id uuid primary key default gen_random_uuid(),
	group_id uuid not null references spendit.expense_groups(id) on delete cascade,
	name text not null,
	normalized_name text not null,
	usage_count integer not null default 0,
	last_used_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create table if not exists spendit.expense_merchant_category_maps (
	id uuid primary key default gen_random_uuid(),
	group_id uuid not null references spendit.expense_groups(id) on delete cascade,
	normalized_merchant text not null,
	entry_type text not null default 'expense' check (entry_type in ('expense', 'income')),
	category_id uuid not null references spendit.expense_categories(id) on delete cascade,
	confidence numeric(4,3) not null default 1.000,
	source text not null default 'learned',
	hit_count integer not null default 0,
	last_seen_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create table if not exists spendit.expense_category_rules (
	id uuid primary key default gen_random_uuid(),
	group_id uuid not null references spendit.expense_groups(id) on delete cascade,
	priority integer not null default 100,
	enabled boolean not null default true,
	entry_type text not null default 'any' check (entry_type in ('expense', 'income', 'any')),
	match_field text not null default 'merchant' check (match_field in ('merchant', 'note', 'account_type')),
	match_kind text not null default 'contains' check (match_kind in ('contains', 'prefix', 'equals', 'regex')),
	pattern text not null,
	category_id uuid not null references spendit.expense_categories(id) on delete cascade,
	confidence numeric(4,3) not null default 0.900,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

-- Statement ingestion stores only normalized fields required for review. Raw PDF
-- bytes and extracted page text are intentionally local-client data.
create table if not exists spendit.expense_statement_ingestions (
	id uuid primary key default gen_random_uuid(),
	group_id uuid not null references spendit.expense_groups(id) on delete cascade,
	account_id uuid not null references spendit.expense_accounts(id) on delete restrict,
	client_request_id text not null default '',
	source_fingerprint text not null default '',
	status text not null check (status in ('parsed', 'matching_review', 'ready', 'confirmed', 'failed', 'deleted')),
	source_name text not null,
	institution text not null default '',
	statement_currency text not null default 'SGD',
	statement_date date,
	payment_due_date date,
	period_start date,
	period_end date,
	previous_balance integer,
	declared_new_transactions_total integer,
	statement_grand_total integer,
	parsed_row_count integer not null default 0,
	cardholder_controls jsonb not null default '[]'::jsonb,
	validation jsonb not null default '{}'::jsonb,
	warnings jsonb not null default '[]'::jsonb,
	created_by uuid references spendit.expense_users(id) on delete set null,
	confirmed_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create table if not exists spendit.expense_statement_ingestion_rows (
	id uuid primary key default gen_random_uuid(),
	ingestion_id uuid not null references spendit.expense_statement_ingestions(id) on delete cascade,
	group_id uuid not null references spendit.expense_groups(id) on delete cascade,
	source_row_key text not null,
	occurred_on date,
	merchant text not null default '',
	amount integer not null default 0 check (amount >= 0),
	currency text not null default 'SGD',
	foreign_amount integer check (foreign_amount is null or foreign_amount >= 0),
	foreign_currency text not null default '',
	statement_kind text not null default 'transaction' check (statement_kind in ('transaction', 'payment', 'fee', 'refund', 'other')),
	type text not null default 'expense' check (type in ('expense', 'income')),
	cardholder text not null default '',
	statement_reference text not null default '',
	account_id uuid references spendit.expense_accounts(id) on delete set null,
	category_id uuid references spendit.expense_categories(id) on delete set null,
	note text not null default '',
	review_status text not null default 'unreviewed' check (review_status in ('unreviewed', 'new', 'matched', 'ignored')),
	suggested_expense_id uuid references spendit.expense_entries(id) on delete set null,
	match_confidence numeric(4,3),
	match_expense_id uuid references spendit.expense_entries(id) on delete set null,
	combined_match_id uuid,
	combined_transaction jsonb,
	confirmed_expense_id uuid references spendit.expense_entries(id) on delete set null,
	warning_codes jsonb not null default '[]'::jsonb,
	reviewed_by uuid references spendit.expense_users(id) on delete set null,
	reviewed_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create unique index if not exists expense_accounts_group_name_uidx
on spendit.expense_accounts (group_id, lower(name))
where deleted_at is null;

create unique index if not exists expense_categories_group_name_uidx
on spendit.expense_categories (group_id, scope, coalesce(owner_user_id, '00000000-0000-0000-0000-000000000000'::uuid), lower(name))
where deleted_at is null;

create unique index if not exists expense_merchants_group_normalized_name_uidx
on spendit.expense_merchants (group_id, normalized_name);

create unique index if not exists expense_merchant_category_maps_group_merchant_type_uidx
on spendit.expense_merchant_category_maps (group_id, normalized_merchant, entry_type);

create index if not exists expense_merchant_category_maps_merchant_trgm_idx
on spendit.expense_merchant_category_maps using gin (normalized_merchant gin_trgm_ops);

create index if not exists expense_category_rules_group_priority_idx
on spendit.expense_category_rules (group_id, priority, updated_at desc)
where deleted_at is null and enabled = true;

create index if not exists expense_entries_group_period_idx
on spendit.expense_entries (group_id, occurred_on desc)
where deleted_at is null;

create index if not exists expense_entries_group_category_period_idx
on spendit.expense_entries (group_id, category_id, occurred_on desc)
where deleted_at is null;

create index if not exists expense_entries_group_account_period_idx
on spendit.expense_entries (group_id, account_id, occurred_on desc)
where deleted_at is null;

create index if not exists expense_statement_ingestions_group_updated_idx
on spendit.expense_statement_ingestions (group_id, updated_at desc)
where deleted_at is null;

create unique index if not exists expense_statement_ingestions_client_request_uidx
on spendit.expense_statement_ingestions (group_id, client_request_id)
where deleted_at is null and client_request_id <> '';

create unique index if not exists expense_statement_ingestions_fingerprint_uidx
on spendit.expense_statement_ingestions (group_id, source_fingerprint)
where deleted_at is null and source_fingerprint <> '';

create unique index if not exists expense_statement_ingestion_rows_source_uidx
on spendit.expense_statement_ingestion_rows (ingestion_id, source_row_key)
where deleted_at is null;

-- Ordinary matches remain one-to-one. Rows intentionally grouped with a
-- combined_match_id are allowed to share the selected master transaction.
create unique index if not exists expense_statement_ingestion_rows_single_match_uidx
on spendit.expense_statement_ingestion_rows (ingestion_id, match_expense_id)
where deleted_at is null and match_expense_id is not null and combined_match_id is null;

create index if not exists expense_statement_ingestion_rows_combined_match_idx
on spendit.expense_statement_ingestion_rows (ingestion_id, combined_match_id)
where deleted_at is null and combined_match_id is not null;

create index if not exists expense_statement_ingestion_rows_review_idx
on spendit.expense_statement_ingestion_rows (ingestion_id, review_status, occurred_on)
where deleted_at is null;

create index if not exists expense_statement_ingestion_rows_suggestion_idx
on spendit.expense_statement_ingestion_rows (ingestion_id, suggested_expense_id)
where deleted_at is null and suggested_expense_id is not null;

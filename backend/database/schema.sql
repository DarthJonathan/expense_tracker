create extension if not exists pgcrypto;
create extension if not exists pg_trgm;
create extension if not exists unaccent;
create schema if not exists spendit;

create table if not exists spendit.expense_users (
	id uuid primary key default gen_random_uuid(),
	email text not null unique,
	password_hash text not null,
	display_name text not null,
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

create unique index if not exists expense_accounts_group_name_uidx
on spendit.expense_accounts (group_id, lower(name))
where deleted_at is null;

create unique index if not exists expense_categories_group_name_uidx
on spendit.expense_categories (group_id, scope, coalesce(owner_user_id, '00000000-0000-0000-0000-000000000000'::uuid), lower(name))
where deleted_at is null;

create unique index if not exists expense_merchants_group_normalized_name_uidx
on spendit.expense_merchants (group_id, normalized_name);

create unique index if not exists expense_merchant_category_maps_group_merchant_type_uidx
on spendit.expense_merchant_category_maps (group_id, normalized_merchant, entry_type)
where deleted_at is null;

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

create extension if not exists pgcrypto;

create table if not exists public.expense_users (
	id uuid primary key default gen_random_uuid(),
	email text not null unique,
	password_hash text not null,
	display_name text not null,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create table if not exists public.expense_groups (
	id uuid primary key default gen_random_uuid(),
	name text not null,
	invite_code text not null unique,
	created_by uuid,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create table if not exists public.expense_accounts (
	id uuid primary key default gen_random_uuid(),
	group_id uuid not null references public.expense_groups(id) on delete cascade,
	name text not null,
	type text not null check (type in ('cash', 'bank', 'card', 'wallet')),
	opening_balance integer not null default 0,
	color text not null default '#4b5745',
	icon text not null default '🏦',
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create table if not exists public.expense_categories (
	id uuid primary key default gen_random_uuid(),
	group_id uuid not null references public.expense_groups(id) on delete cascade,
	name text not null,
	type text not null default 'expense' check (type in ('expense', 'income')),
	color text not null default '#e7d24e',
	icon text not null default '🏷️',
	monthly_target integer not null default 0,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create table if not exists public.expense_entries (
	id uuid primary key default gen_random_uuid(),
	group_id uuid not null references public.expense_groups(id) on delete cascade,
	account_id uuid not null references public.expense_accounts(id) on delete restrict,
	category_id uuid not null references public.expense_categories(id) on delete restrict,
	type text not null check (type in ('expense', 'income')),
	amount integer not null check (amount >= 0),
	currency text not null default 'USD',
	occurred_on date not null,
	merchant text not null,
	note text not null default '',
	created_by uuid,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create table if not exists public.expense_category_adjustments (
	id uuid primary key default gen_random_uuid(),
	group_id uuid not null references public.expense_groups(id) on delete cascade,
	category_id uuid not null references public.expense_categories(id) on delete cascade,
	amount integer not null,
	occurred_on date not null,
	note text not null default '',
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create table if not exists public.expense_merchants (
	id uuid primary key default gen_random_uuid(),
	group_id uuid not null references public.expense_groups(id) on delete cascade,
	name text not null,
	normalized_name text not null,
	usage_count integer not null default 0,
	last_used_at timestamptz,
	created_at timestamptz not null default now(),
	updated_at timestamptz not null default now(),
	deleted_at timestamptz
);

create unique index if not exists expense_accounts_group_name_uidx
on public.expense_accounts (group_id, lower(name))
where deleted_at is null;

create unique index if not exists expense_categories_group_name_uidx
on public.expense_categories (group_id, lower(name))
where deleted_at is null;

create unique index if not exists expense_merchants_group_normalized_name_uidx
on public.expense_merchants (group_id, normalized_name);

create index if not exists expense_entries_group_period_idx
on public.expense_entries (group_id, occurred_on desc)
where deleted_at is null;

create index if not exists expense_entries_group_category_period_idx
on public.expense_entries (group_id, category_id, occurred_on desc)
where deleted_at is null;

create index if not exists expense_entries_group_account_period_idx
on public.expense_entries (group_id, account_id, occurred_on desc)
where deleted_at is null;

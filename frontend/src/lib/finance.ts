import { get, writable } from 'svelte/store';
import { putRecord, loadFinanceState, patchSettings } from './db';
import { syncFinanceState } from './sync';
import type {
	Account,
	AccountType,
	Category,
	CategoryScope,
	CategoryType,
	CategoryAdjustment,
	EntryType,
	FinanceState,
	Group,
	LedgerEntry,
	Merchant
} from './types';
import { cents, isoNow, makeId, normalizeText, todayInputValue } from './utils';

interface SyncStatus {
	state: 'idle' | 'syncing' | 'offline' | 'error';
	message: string;
}

const financeState = writable<FinanceState | null>(null);
const syncStatus = writable<SyncStatus>({ state: 'idle', message: 'Local changes saved' });

function touch<T extends { updatedAt: string; deletedAt?: string | null }>(record: T): T {
	return { ...record, updatedAt: isoNow(), deletedAt: record.deletedAt ?? null };
}

function updateList<T extends { id: string }>(records: T[], record: T): T[] {
	const exists = records.some((item) => item.id === record.id);
	return exists ? records.map((item) => (item.id === record.id ? record : item)) : [record, ...records];
}

function normalizeMerchantKey(value: string): string {
	return value
		.toLowerCase()
		.trim()
		.replace(/\s+/g, ' ');
}

async function mutate<K extends keyof FinanceState>(
	key: K,
	storeName: Parameters<typeof putRecord>[0],
	record: FinanceState[K] extends Array<infer R> ? R & { id: string } : never
): Promise<void> {
	financeState.update((state) => {
		if (!state) return state;
		return { ...state, [key]: updateList(state[key] as never[], record as never) };
	});

	await putRecord(storeName, record);
}

export const finance = {
	subscribe: financeState.subscribe,
	syncStatus,

	async init() {
		const loaded = await loadFinanceState();
		const normalizedEntries = loaded.entries.map((entry) =>
			entry.currency === 'SGD' ? entry : { ...entry, currency: 'SGD', updatedAt: isoNow() }
		);
		financeState.set({ ...loaded, entries: normalizedEntries });

		if (normalizedEntries.some((entry, index) => entry !== loaded.entries[index])) {
			await Promise.all(normalizedEntries.map((entry) => putRecord('entries', entry)));
		}
	},

	async addEntry(formData: FormData) {
		const state = get(financeState);
		if (!state) return;

		const groupId = state.settings.activeGroupId;
		const now = isoNow();
		const type = normalizeText(formData.get('type'), 'expense') as EntryType;
		const categoryId = normalizeText(formData.get('categoryId'));
		const category = state.categories.find((item) => {
			if (item.id !== categoryId || item.groupId !== groupId || item.deletedAt) return false;
			const scope = normalizeCategoryScope(item.scope);
			if (scope === 'user' && item.ownerUserId && item.ownerUserId !== state.settings.deviceUserId) return false;
			return true;
		});
		if (!category) {
			throw new Error('Category not found for this household.');
		}
		const categoryType = normalizeCategoryType(category.type, category.name);
		if (categoryType !== type) {
			throw new Error(`Category "${category.name}" is ${categoryType}, but transaction is ${type}.`);
		}
		const entry: LedgerEntry = {
			id: makeId(),
			groupId,
			accountId: normalizeText(formData.get('accountId')),
			categoryId,
			type,
			amount: cents(formData.get('amount')),
			currency: 'SGD',
			occurredOn: normalizeText(formData.get('occurredOn'), todayInputValue()),
			merchant: normalizeText(formData.get('merchant'), 'Untitled'),
			note: normalizeText(formData.get('note')),
			createdBy: state.settings.deviceUserId,
			createdAt: now,
			updatedAt: now,
			deletedAt: null
		};

		await mutate('entries', 'entries', entry);
		await upsertMerchantRecord(state, entry.merchant, entry.occurredOn);
	},

	async updateEntry(entryId: string, formData: FormData) {
		const state = get(financeState);
		if (!state) return;

		const existing = state.entries.find((entry) => entry.id === entryId && !entry.deletedAt);
		if (!existing) {
			throw new Error('Transaction not found.');
		}

		const groupId = state.settings.activeGroupId;
		const type = normalizeText(formData.get('type'), existing.type) as EntryType;
		const categoryId = normalizeText(formData.get('categoryId'), existing.categoryId);
		const category = state.categories.find((item) => {
			if (item.id !== categoryId || item.groupId !== groupId || item.deletedAt) return false;
			const scope = normalizeCategoryScope(item.scope);
			if (scope === 'user' && item.ownerUserId && item.ownerUserId !== state.settings.deviceUserId) return false;
			return true;
		});

		if (!category) {
			throw new Error('Category not found for this household.');
		}

		const categoryType = normalizeCategoryType(category.type, category.name);
		if (categoryType !== type) {
			throw new Error(`Category "${category.name}" is ${categoryType}, but transaction is ${type}.`);
		}

		const next: LedgerEntry = touch({
			...existing,
			accountId: normalizeText(formData.get('accountId'), existing.accountId),
			categoryId,
			type,
			amount: cents(formData.get('amount')),
			currency: 'SGD',
			occurredOn: normalizeText(formData.get('occurredOn'), existing.occurredOn),
			merchant: normalizeText(formData.get('merchant'), existing.merchant),
			note: normalizeText(formData.get('note'), existing.note)
		});

		await mutate('entries', 'entries', next);
		await upsertMerchantRecord(state, next.merchant, next.occurredOn);
	},

	async addAccount(formData: FormData) {
		const state = get(financeState);
		if (!state) return;

		const now = isoNow();
		const account: Account = {
			id: makeId(),
			groupId: state.settings.activeGroupId,
			name: normalizeText(formData.get('name'), 'New account'),
			type: normalizeText(formData.get('type'), 'bank') as AccountType,
			openingBalance: cents(formData.get('openingBalance')),
			color: normalizeText(formData.get('color'), '#4b5745'),
			icon: normalizeText(formData.get('icon'), accountIconByType(normalizeText(formData.get('type'), 'bank') as AccountType)),
			createdAt: now,
			updatedAt: now,
			deletedAt: null
		};

		await mutate('accounts', 'accounts', account);
	},

	async updateAccount(accountId: string, formData: FormData) {
		const state = get(financeState);
		if (!state) return;

		const existing = state.accounts.find((account) => account.id === accountId && !account.deletedAt);
		if (!existing) return;

		const type = normalizeText(formData.get('type'), existing.type) as AccountType;
		const next: Account = touch({
			...existing,
			name: normalizeText(formData.get('name'), existing.name),
			type,
			openingBalance: cents(formData.get('openingBalance')),
			color: normalizeText(formData.get('color'), existing.color),
			icon: normalizeText(formData.get('icon'), accountIconByType(type))
		});

		await mutate('accounts', 'accounts', next);
	},

	async addCategory(formData: FormData) {
		const state = get(financeState);
		if (!state) return;

		const now = isoNow();
		const scope = normalizeCategoryScope(normalizeText(formData.get('scope'), 'household'));
		const category: Category = {
			id: makeId(),
			groupId: state.settings.activeGroupId,
			name: normalizeText(formData.get('name'), 'New category'),
			type: normalizeCategoryType(
				normalizeText(formData.get('type')),
				normalizeText(formData.get('name'), 'New category')
			),
			scope,
			ownerUserId: scope === 'user' ? state.settings.deviceUserId : null,
			color: normalizeText(formData.get('color'), '#e7d24e'),
			icon: normalizeText(formData.get('icon'), categoryIconByName(normalizeText(formData.get('name'), 'Category'))),
			monthlyTarget: cents(formData.get('monthlyTarget')),
			createdAt: now,
			updatedAt: now,
			deletedAt: null
		};

		await mutate('categories', 'categories', category);
	},

	async updateCategory(categoryId: string, formData: FormData) {
		const state = get(financeState);
		if (!state) return;

		const existing = state.categories.find((category) => category.id === categoryId && !category.deletedAt);
		if (!existing) return;

		const name = normalizeText(formData.get('name'), existing.name);
		const type = normalizeCategoryType(normalizeText(formData.get('type')), name);
		const scope = normalizeCategoryScope(normalizeText(formData.get('scope'), existing.scope ?? 'household'));
		const next: Category = touch({
			...existing,
			name,
			type,
			scope,
			ownerUserId: scope === 'user' ? existing.ownerUserId ?? state.settings.deviceUserId : null,
			color: normalizeText(formData.get('color'), existing.color),
			icon: normalizeText(formData.get('icon'), categoryIconByName(name)),
			monthlyTarget: cents(formData.get('monthlyTarget'))
		});

		await mutate('categories', 'categories', next);
	},

	async addAdjustment(formData: FormData) {
		const state = get(financeState);
		if (!state) return;

		const now = isoNow();
		const adjustment: CategoryAdjustment = {
			id: makeId(),
			groupId: state.settings.activeGroupId,
			categoryId: normalizeText(formData.get('categoryId')),
			amount: cents(formData.get('amount')),
			occurredOn: normalizeText(formData.get('occurredOn'), todayInputValue()),
			note: normalizeText(formData.get('note'), 'Manual category adjustment'),
			createdAt: now,
			updatedAt: now,
			deletedAt: null
		};

		await mutate('adjustments', 'adjustments', adjustment);
	},

	async updateGroupName(formData: FormData) {
		const state = get(financeState);
		if (!state) return;

		const group = state.groups.find((item) => item.id === state.settings.activeGroupId);
		if (!group) return;

		const next: Group = touch({ ...group, name: normalizeText(formData.get('name'), group.name) });
		await mutate('groups', 'groups', next);
	},

	async syncNow(): Promise<boolean> {
		if (!navigator.onLine) {
			syncStatus.set({ state: 'offline', message: 'Offline. Changes will sync when connection returns.' });
			return false;
		}

		const state = get(financeState);
		if (!state) return false;

		try {
			syncStatus.set({ state: 'syncing', message: 'Syncing with backend API...' });
			const synced = await syncFinanceState(state);
			await patchSettings({
				activeGroupId: synced.settings.activeGroupId,
				lastSyncedAt: synced.settings.lastSyncedAt
			});
			financeState.set(synced);
			syncStatus.set({ state: 'idle', message: 'Synced' });
			return true;
		} catch (error) {
			syncStatus.set({
				state: 'error',
				message: error instanceof Error ? error.message : 'Sync failed'
			});
			return false;
		}
	}
};

async function upsertMerchantRecord(state: FinanceState, merchantName: string, occurredOn: string): Promise<void> {
	const normalizedName = normalizeMerchantKey(merchantName);
	if (!normalizedName) return;

	const now = isoNow();
	const usedAt = `${occurredOn}T00:00:00.000Z`;
	const existing = state.merchants.find(
		(merchant) => merchant.groupId === state.settings.activeGroupId && merchant.normalizedName === normalizedName && !merchant.deletedAt
	);

	const next: Merchant = existing
		? {
				...existing,
				name: merchantName,
				usageCount: (existing.usageCount ?? 0) + 1,
				lastUsedAt: usedAt,
				updatedAt: now,
				deletedAt: null
			}
		: {
				id: makeId(),
				groupId: state.settings.activeGroupId,
				name: merchantName,
				normalizedName,
				usageCount: 1,
				lastUsedAt: usedAt,
				createdAt: now,
				updatedAt: now,
				deletedAt: null
			};

	financeState.update((current) => {
		if (!current) return current;
		return { ...current, merchants: updateList(current.merchants, next) };
	});

	await putRecord('merchants', next);
}

function accountIconByType(type: AccountType): string {
	switch (type) {
		case 'cash':
			return '💵';
		case 'card':
			return '💳';
		case 'wallet':
			return '👛';
		case 'bank':
		default:
			return '🏦';
	}
}

function categoryIconByName(name: string): string {
	const normalized = name.toLowerCase();
	if (normalized.includes('grocer') || normalized.includes('food') || normalized.includes('eat')) return '🍽️';
	if (normalized.includes('transport') || normalized.includes('fuel') || normalized.includes('car')) return '🚗';
	if (normalized.includes('home') || normalized.includes('rent')) return '🏠';
	if (normalized.includes('health') || normalized.includes('medic')) return '🩺';
	if (normalized.includes('income') || normalized.includes('salary')) return '💼';
	return '🏷️';
}

function normalizeCategoryType(value: string, nameFallback: string): CategoryType {
	const normalized = value.toLowerCase();
	if (normalized === 'income' || normalized === 'expense') {
		return normalized;
	}

	const inferred = nameFallback.toLowerCase();
	if (inferred.includes('income') || inferred.includes('salary') || inferred.includes('payroll')) {
		return 'income';
	}
	return 'expense';
}

function normalizeCategoryScope(value: string): CategoryScope {
	return value.toLowerCase().trim() === 'user' ? 'user' : 'household';
}

import { get, writable } from 'svelte/store';
import { putRecord, loadFinanceState, patchSettings } from './db';
import { syncFinanceState } from './sync';
import type {
	Account,
	AccountType,
	Category,
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
		financeState.set(await loadFinanceState());
	},

	async addEntry(formData: FormData) {
		const state = get(financeState);
		if (!state) return;

		const groupId = state.settings.activeGroupId;
		const now = isoNow();
		const entry: LedgerEntry = {
			id: makeId(),
			groupId,
			accountId: normalizeText(formData.get('accountId')),
			categoryId: normalizeText(formData.get('categoryId')),
			type: normalizeText(formData.get('type'), 'expense') as EntryType,
			amount: cents(formData.get('amount')),
			currency: normalizeText(formData.get('currency'), 'USD').toUpperCase(),
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

	async addCategory(formData: FormData) {
		const state = get(financeState);
		if (!state) return;

		const now = isoNow();
		const category: Category = {
			id: makeId(),
			groupId: state.settings.activeGroupId,
			name: normalizeText(formData.get('name'), 'New category'),
			type: normalizeCategoryType(
				normalizeText(formData.get('type')),
				normalizeText(formData.get('name'), 'New category')
			),
			color: normalizeText(formData.get('color'), '#e7d24e'),
			icon: normalizeText(formData.get('icon'), categoryIconByName(normalizeText(formData.get('name'), 'Category'))),
			monthlyTarget: cents(formData.get('monthlyTarget')),
			createdAt: now,
			updatedAt: now,
			deletedAt: null
		};

		await mutate('categories', 'categories', category);
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
			await patchSettings({ lastSyncedAt: synced.settings.lastSyncedAt });
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

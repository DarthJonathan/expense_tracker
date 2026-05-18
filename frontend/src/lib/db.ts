import { createDefaultAccounts, createDefaultCategories, createDefaultGroup } from './constants';
import type {
	Account,
	AppSettings,
	Category,
	CategoryAdjustment,
	FinanceState,
	Group,
	LedgerEntry,
	Merchant
} from './types';
import { isoNow, makeId } from './utils';

type StoreName = 'settings' | 'groups' | 'accounts' | 'categories' | 'entries' | 'adjustments' | 'merchants';

const DB_NAME = 'shared-expense-tracker';
const DB_VERSION = 2;

let dbPromise: Promise<IDBDatabase> | undefined;

function openDb(): Promise<IDBDatabase> {
	if (dbPromise) return dbPromise;

	dbPromise = new Promise((resolve, reject) => {
		const request = indexedDB.open(DB_NAME, DB_VERSION);

		request.onupgradeneeded = () => {
			const db = request.result;
			for (const storeName of ['settings', 'groups', 'accounts', 'categories', 'entries', 'adjustments', 'merchants']) {
				if (!db.objectStoreNames.contains(storeName)) {
					db.createObjectStore(storeName, { keyPath: 'id' });
				}
			}
		};

		request.onsuccess = () => resolve(request.result);
		request.onerror = () => reject(request.error);
	});

	return dbPromise;
}

async function tx<T>(storeName: StoreName, mode: IDBTransactionMode, run: (store: IDBObjectStore) => IDBRequest<T>): Promise<T> {
	const db = await openDb();

	return new Promise((resolve, reject) => {
		const transaction = db.transaction(storeName, mode);
		const request = run(transaction.objectStore(storeName));

		request.onsuccess = () => resolve(request.result);
		request.onerror = () => reject(request.error);
		transaction.onerror = () => reject(transaction.error);
	});
}

export async function getAll<T>(storeName: StoreName): Promise<T[]> {
	return tx<T[]>(storeName, 'readonly', (store) => store.getAll());
}

export async function putRecord<T extends { id: string }>(storeName: StoreName, record: T): Promise<void> {
	await tx<IDBValidKey>(storeName, 'readwrite', (store) => store.put(record));
}

export async function putMany<T extends { id: string }>(storeName: StoreName, records: T[]): Promise<void> {
	const db = await openDb();

	await new Promise<void>((resolve, reject) => {
		const transaction = db.transaction(storeName, 'readwrite');
		const store = transaction.objectStore(storeName);

		for (const record of records) {
			store.put(record);
		}

		transaction.oncomplete = () => resolve();
		transaction.onerror = () => reject(transaction.error);
	});
}

export async function patchSettings(patch: Partial<AppSettings>): Promise<AppSettings> {
	const settings = await getSettings();
	const next = { ...settings, ...patch };
	await putRecord('settings', next);
	return next;
}

export async function getSettings(): Promise<AppSettings> {
	const existing = await tx<AppSettings | undefined>('settings', 'readonly', (store) => store.get('settings'));
	if (existing) return existing;

	const group = createDefaultGroup();
	const accounts = createDefaultAccounts(group.id);
	const categories = createDefaultCategories(group.id);
	const settings: AppSettings = {
		id: 'settings',
		activeGroupId: group.id,
		deviceUserId: makeId(),
		lastSyncedAt: null
	};

	await putRecord('groups', group);
	await putMany('accounts', accounts);
	await putMany('categories', categories);
	await putRecord('settings', settings);

	return settings;
}

export async function loadFinanceState(): Promise<FinanceState> {
	const settings = await getSettings();
	const [groups, accounts, categories, entries, adjustments, merchants] = await Promise.all([
		getAll<Group>('groups'),
		getAll<Account>('accounts'),
		getAll<Category>('categories'),
		getAll<LedgerEntry>('entries'),
		getAll<CategoryAdjustment>('adjustments'),
		getAll<Merchant>('merchants')
	]);

	return { settings, groups, accounts, categories, entries, adjustments, merchants };
}

export async function mergeRemoteState(remote: Partial<Omit<FinanceState, 'settings'>>): Promise<void> {
	await Promise.all([
		remote.groups?.length ? putMany('groups', remote.groups) : Promise.resolve(),
		remote.accounts?.length ? putMany('accounts', remote.accounts) : Promise.resolve(),
		remote.categories?.length ? putMany('categories', remote.categories) : Promise.resolve(),
		remote.entries?.length ? putMany('entries', remote.entries) : Promise.resolve(),
		remote.adjustments?.length ? putMany('adjustments', remote.adjustments) : Promise.resolve(),
		remote.merchants?.length ? putMany('merchants', remote.merchants) : Promise.resolve()
	]);

	await patchSettings({ lastSyncedAt: isoNow() });
}

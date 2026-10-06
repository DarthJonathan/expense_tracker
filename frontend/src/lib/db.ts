import { createDefaultAccounts, createDefaultCategories, createDefaultGroup } from './constants';
import type {
	Account,
	AppSettings,
	Category,
	CategoryAdjustment,
	FinanceState,
	Group,
	LedgerEntry,
	LocalStatementJob,
	Merchant
} from './types';
import { makeId } from './utils';
import { sameSyncRecord, SYNC_COLLECTIONS, type SyncCheckpoint, type SyncRecords, type SyncRemovals } from './sync-protocol';

type StoreName =
	| 'settings'
	| 'groups'
	| 'accounts'
	| 'categories'
	| 'entries'
	| 'adjustments'
	| 'merchants'
	| 'syncCheckpoints'
	| 'statementLocalJobs';

const DB_NAME = 'shared-expense-tracker';
const DB_VERSION = 4;

let dbPromise: Promise<IDBDatabase> | undefined;

function openDb(): Promise<IDBDatabase> {
	if (dbPromise) return dbPromise;

	dbPromise = new Promise((resolve, reject) => {
		const request = indexedDB.open(DB_NAME, DB_VERSION);

		request.onupgradeneeded = () => {
			const db = request.result;
			for (const storeName of [
				'settings',
				'groups',
				'accounts',
				'categories',
				'entries',
				'adjustments',
				'merchants',
				'syncCheckpoints',
				'statementLocalJobs'
			]) {
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

export async function deleteRecord(storeName: StoreName, id: string): Promise<void> {
	await tx<undefined>(storeName, 'readwrite', (store) => store.delete(id));
}

export async function getLocalStatementJobs(groupId: string): Promise<LocalStatementJob[]> {
	const jobs = await getAll<LocalStatementJob>('statementLocalJobs');
	return jobs.filter((job) => job.groupId === groupId).sort((a, b) => b.updatedAt.localeCompare(a.updatedAt));
}

export async function putLocalStatementJob(job: LocalStatementJob): Promise<void> {
	await putRecord('statementLocalJobs', job);
}

export async function deleteLocalStatementJob(id: string): Promise<void> {
	await deleteRecord('statementLocalJobs', id);
}

export async function patchSettings(patch: Partial<AppSettings>): Promise<AppSettings> {
	const settings = await getSettings();
	const next = { ...settings, ...patch };
	await putRecord('settings', next);
	return next;
}

export async function getSettings(): Promise<AppSettings> {
	const existing = await tx<AppSettings | undefined>('settings', 'readonly', (store) => store.get('settings'));
	if (existing) {
		const baseCurrency = existing.baseCurrency?.trim().toUpperCase() || 'SGD';
		if (existing.baseCurrency !== baseCurrency) {
			const normalized = { ...existing, baseCurrency };
			await putRecord('settings', normalized);
			return normalized;
		}
		return existing;
	}

	const group = createDefaultGroup();
	const accounts = createDefaultAccounts(group.id);
	const categories = createDefaultCategories(group.id);
	const settings: AppSettings = {
		id: 'settings',
		activeGroupId: group.id,
		deviceUserId: makeId(),
		baseCurrency: 'SGD',
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

export async function getSyncCheckpoint(id: string): Promise<SyncCheckpoint> {
	return (await tx<SyncCheckpoint | undefined>('syncCheckpoints', 'readonly', (store) => store.get(id)))
		?? { id, fingerprints: {} };
}

export async function saveSyncCheckpoint(checkpoint: SyncCheckpoint): Promise<void> {
	// Wait for the transaction to commit before considering a batch acknowledged.
	await putMany('syncCheckpoints', [checkpoint]);
}

async function mergeRemoteRecords(storeName: StoreName, records: Array<{ id: string; updatedAt: string }>, expected: Array<{ id: string; updatedAt: string }>, removed: string[]): Promise<void> {
	const db = await openDb();
	await new Promise<void>((resolve, reject) => {
		const transaction = db.transaction(storeName, 'readwrite');
		const store = transaction.objectStore(storeName);
		const before = new Map(expected.map((record) => [record.id, record]));
		for (const record of records) {
			const request = store.get(record.id);
			request.onsuccess = () => {
				// Server authority is independent of clocks. Only overwrite the exact
				// local snapshot approved by sync; later edits remain pending.
				if (sameSyncRecord(request.result, before.get(record.id))) {
					store.put(record);
				}
			};
		}
		for (const id of removed) {
			const request = store.get(id);
			request.onsuccess = () => { if (sameSyncRecord(request.result, before.get(id))) store.delete(id); };
		}
		transaction.oncomplete = () => resolve();
		transaction.onerror = () => reject(transaction.error);
		transaction.onabort = () => reject(transaction.error);
	});
}

export async function mergeRemoteState(remote: SyncRecords, expected: SyncRecords, removals: SyncRemovals = {}): Promise<void> {
	await Promise.all(SYNC_COLLECTIONS.map((collection) => remote[collection].length || removals[collection]?.length
		? mergeRemoteRecords(collection, remote[collection], expected[collection], removals[collection] ?? []) : Promise.resolve()));
}

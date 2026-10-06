import type { FinanceState } from './types';

// Dependencies must reach the server before transactions and adjustments.
export const SYNC_COLLECTIONS = ['groups', 'accounts', 'categories', 'entries', 'adjustments', 'merchants'] as const;
export type SyncCollection = typeof SYNC_COLLECTIONS[number];
export type SyncRecords = Pick<FinanceState, SyncCollection>;
export type SyncRecord = SyncRecords[SyncCollection][number];
export type SyncVersions = Partial<Record<SyncCollection, Record<string, string>>>;
export type SyncRemovals = Partial<Record<SyncCollection, string[]>>;
export interface SyncConflict {
	collection: SyncCollection;
	id: string;
	serverVersion: string;
	server: SyncRecord | null;
	local: SyncRecord;
}
export type SyncResolution = 'server' | 'local' | 'later';
export class SyncConflictDeferredError extends Error {
	constructor() { super('Sync needs a conflict decision. Your local edits are saved. Retry sync to review.'); }
}
export interface SyncCheckpoint {
	id: string;
	fingerprints: Partial<Record<SyncCollection, Record<string, string>>>;
	versions?: Partial<Record<SyncCollection, Record<string, string>>>;
	baseVersions?: SyncVersions;
}
export interface SyncOptions {
	mode: 'push' | 'pull';
	collection?: SyncCollection;
	cursor?: string;
	limit?: number;
	known?: Record<string, string>;
	version?: number;
	baseVersions?: SyncVersions;
}
export type SyncPayload = SyncRecords & { settings: FinanceState['settings']; sync?: SyncOptions };
export const MAX_SYNC_BYTES = 128 * 1024;
export const MAX_SYNC_RECORDS = 100;

export function emptySyncRecords(): SyncRecords {
	return { groups: [], accounts: [], categories: [], entries: [], adjustments: [], merchants: [] };
}

function canonicalJSON(value: unknown): string {
	if (Array.isArray(value)) return `[${value.map(canonicalJSON).join(',')}]`;
	if (value && typeof value === 'object') {
		return `{${Object.entries(value).filter(([, item]) => item !== undefined).sort(([a], [b]) => a.localeCompare(b))
			.map(([key, item]) => `${JSON.stringify(key)}:${canonicalJSON(item)}`).join(',')}}`;
	}
	return JSON.stringify(value) ?? 'null';
}

export function sameSyncRecord(left: unknown, right: unknown): boolean {
	return canonicalJSON(left) === canonicalJSON(right);
}

export function mergeServerState(current: FinanceState, remote: SyncRecords, expected: SyncRecords, removals: SyncRemovals = {}): FinanceState {
	const next = { ...current };
	for (const collection of SYNC_COLLECTIONS) {
		const records = new Map<string, SyncRecord>(current[collection].map((record) => [record.id, record]));
		const before = new Map<string, SyncRecord>(expected[collection].map((record) => [record.id, record]));
		const unchanged = (id: string) => sameSyncRecord(records.get(id), before.get(id));
		for (const record of remote[collection]) {
			if (unchanged(record.id)) records.set(record.id, record);
		}
		for (const id of removals[collection] ?? []) if (unchanged(id)) records.delete(id);
		next[collection] = [...records.values()] as never;
	}
	return next;
}

export async function recordFingerprint(record: unknown): Promise<string> {
	const serialized = canonicalJSON(record);
	// Local HTTP development may not expose SubtleCrypto. Preserve exact equality there.
	if (!globalThis.crypto?.subtle) return serialized;
	const hash = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(serialized));
	return Array.from(new Uint8Array(hash), (byte) => byte.toString(16).padStart(2, '0')).join('');
}

export async function pendingSyncRecords(records: SyncRecords, checkpoint: SyncCheckpoint): Promise<SyncRecords> {
	const pending = emptySyncRecords();
	for (const collection of SYNC_COLLECTIONS) {
		for (const record of records[collection]) {
			if (checkpoint.fingerprints[collection]?.[record.id] !== await recordFingerprint(record)) {
				(pending[collection] as Array<typeof record>).push(record);
			}
		}
	}
	return pending;
}

export async function acknowledgeSyncRecords(checkpoint: SyncCheckpoint, records: SyncRecords): Promise<void> {
	for (const collection of SYNC_COLLECTIONS) {
		const fingerprints = checkpoint.fingerprints[collection] ??= {};
		for (const record of records[collection]) fingerprints[record.id] = await recordFingerprint(record);
	}
}

export function syncPayloadBytes(payload: SyncPayload): number {
	return new TextEncoder().encode(JSON.stringify(payload)).byteLength;
}

export function* buildSyncBatches(settings: FinanceState['settings'], records: SyncRecords, baseVersions: SyncVersions = {}): Generator<SyncPayload> {
	const createBatch = (): SyncPayload => ({ settings, ...emptySyncRecords(), sync: { mode: 'push', version: 3, baseVersions: {} } });
	const addVersion = (collection: SyncCollection, id: string) => {
		const version = baseVersions[collection]?.[id];
		if (version) (batch.sync!.baseVersions![collection] ??= {})[id] = version;
	};
	let batch = createBatch();
	let count = 0;
	for (const collection of SYNC_COLLECTIONS) {
		for (const record of records[collection]) {
			const list = batch[collection] as Array<typeof record>;
			list.push(record);
			addVersion(collection, record.id);
			if (count >= MAX_SYNC_RECORDS || syncPayloadBytes(batch) > MAX_SYNC_BYTES) {
				list.pop();
				delete batch.sync!.baseVersions?.[collection]?.[record.id];
				if (count) yield batch;
				batch = createBatch();
				(batch[collection] as Array<typeof record>).push(record);
				addVersion(collection, record.id);
				count = 0;
				if (syncPayloadBytes(batch) > MAX_SYNC_BYTES) {
					throw new Error(`Cannot sync ${collection} record ${record.id}: this record exceeds the 128 KB sync limit.`);
				}
			}
			count++;
		}
		// Resolve a parent record's conflict before sending dependent records.
		if (count) { yield batch; batch = createBatch(); count = 0; }
	}
}

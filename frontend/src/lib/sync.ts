import { getSyncCheckpoint, mergeRemoteState, saveSyncCheckpoint } from './db';
import { apiPath } from './api';
import { authFetch, getStoredSession } from './auth';
import { buildSyncBatches, emptySyncRecords, mergeServerState, pendingSyncRecords, recordFingerprint, sameSyncRecord, SYNC_COLLECTIONS, SyncConflictDeferredError, type SyncCollection, type SyncConflict, type SyncPayload, type SyncRecord, type SyncRecords, type SyncRemovals, type SyncResolution, type SyncVersions } from './sync-protocol';
import { readSyncError, syncErrorMessage } from './sync-error';
import type {
	Account,
	Category,
	CategoryAdjustment,
	FinanceState,
	Group,
	LedgerEntry,
	Merchant
} from './types';
import { isoNow, normalizeCurrencyCode } from './utils';

interface SyncResponse {
	protocolVersion?: number;
	conflicts?: Array<Omit<SyncConflict, 'local'>>;
	acceptedVersions?: SyncVersions;
	page?: { collection: string; nextCursor?: string; hasMore: boolean; versions: Record<string, string> };
	settings?: FinanceState['settings'];
	groups: Group[];
	accounts: Account[];
	categories: Category[];
	entries: LedgerEntry[];
	adjustments: CategoryAdjustment[];
	merchants: Merchant[];
	syncedAt: string;
}

interface WrappedSyncResponse {
	success: boolean;
	error?: string;
	data?: SyncResponse;
}

async function syncWithBackend(syncPayload: SyncPayload, userId: string): Promise<SyncResponse> {
	if (getStoredSession()?.user.id !== userId) throw new Error('Your sign-in changed during sync. Please retry.');
	const response = await authFetch(apiPath('/api/v1/sync'), {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(syncPayload)
	});

	if (!response.ok) {
		throw await readSyncError(response);
	}

	const raw = (await response.json()) as SyncResponse | WrappedSyncResponse;
	if ('success' in raw) {
		if (!raw.success) {
			throw new Error(syncErrorMessage(response.status, raw.error, response.headers.get('x-request-id')));
		}
		if (!raw.data) throw new Error('Sync response is missing data.');
		return raw.data;
	}

	return raw;
}

function requireSyncProtocol(data: SyncResponse): void {
	if (data.protocolVersion !== 3) throw new Error('The backend needs an update to support conflict-aware sync.');
}

function responseRecords(data: SyncResponse): SyncRecords {
	const records = emptySyncRecords();
	for (const collection of SYNC_COLLECTIONS) records[collection] = (data[collection] ?? []) as never;
	return records;
}

function singleRecord(collection: SyncCollection, record: SyncRecord): SyncRecords {
	const records = emptySyncRecords();
	(records[collection] as SyncRecord[]).push(record);
	return records;
}

export async function syncFinanceState(
	state: FinanceState,
	getLatestState?: () => FinanceState,
	onProgress: (message: string) => void = () => {},
	onConflict: (conflict: SyncConflict) => Promise<SyncResolution> = async () => { throw new SyncConflictDeferredError(); },
	onApply?: (remote: SyncRecords, expected: SyncRecords, removals: SyncRemovals) => void
): Promise<FinanceState> {
	const userId = getStoredSession()?.user.id;
	if (!userId) throw new Error('Not authenticated');
	const ensureUser = () => { if (getStoredSession()?.user.id !== userId) throw new Error('Your sign-in changed during sync. Please retry.'); };
	const groupId = state.settings.activeGroupId;
	let resolvedGroupId = groupId;
	let working = state;
	let observed = state;
	// Apply only actual external edits to the working state. This also supports
	// callers without a live store while canonical batches are committed in flight.
	const latest = (): FinanceState => {
		const external = getLatestState?.();
		if (external && external !== observed) {
			const next = { ...working, settings: sameSyncRecord(observed.settings, external.settings) ? working.settings : external.settings };
			for (const collection of SYNC_COLLECTIONS) {
				const before = new Map<string, SyncRecord>(observed[collection].map(row => [row.id, row]));
				const current = new Map<string, SyncRecord>(working[collection].map(row => [row.id, row]));
				const ids = new Set(external[collection].map(row => row.id));
				for (const row of external[collection]) if (!sameSyncRecord(before.get(row.id), row)) current.set(row.id, row);
				for (const id of before.keys()) if (!ids.has(id)) current.delete(id);
				next[collection] = [...current.values()] as never;
			}
			working = next;
			observed = external;
		}
		return working;
	};
	const scoped = (): SyncRecords => {
		const records = emptySyncRecords();
		for (const collection of SYNC_COLLECTIONS) records[collection] = latest()[collection].filter(row => collection === 'groups'
			? row.id === resolvedGroupId : 'groupId' in row && row.groupId === resolvedGroupId) as never;
		// IndexedDB survives sign-ins and can contain another household member's
		// private categories. Retain those local records without submitting them
		// as edits or treating their absence from this user's pull as a deletion.
		records.categories = records.categories.filter(category => {
			const owner = category.ownerUserId?.trim();
			return category.scope?.toLowerCase().trim() !== 'user' || !owner || owner === userId;
		});
		return records;
	};
	let checkpoint = await getSyncCheckpoint(`${userId}:${groupId}`);
	checkpoint.baseVersions ??= structuredClone(checkpoint.versions ?? {});
	checkpoint.versions ??= {};
	let backendSettings = state.settings;
	let syncedAt = '';
	let batchNumber = 0;
	const baseVersions = () => checkpoint.baseVersions ?? {};
	const currentRecord = (collection: SyncCollection, id: string) => latest()[collection].find(row => row.id === id);
	const apply = async (remote: SyncRecords, expected: SyncRecords, removals: SyncRemovals = {}) => {
		ensureUser();
		if (!SYNC_COLLECTIONS.some(collection => remote[collection].length || removals[collection]?.length)) return;
		await mergeRemoteState(remote, expected, removals);
		ensureUser();
		working = mergeServerState(latest(), remote, expected, removals);
		onApply?.(remote, expected, removals);
	};
	const rememberServer = async (collection: SyncCollection, record: SyncRecord, version: string, submitted?: SyncRecord) => {
		(checkpoint.baseVersions![collection] ??= {})[record.id] = version;
		const fingerprints = checkpoint.fingerprints[collection] ??= {};
		const cached = checkpoint.versions![collection] ??= {};
		if (sameSyncRecord(currentRecord(collection, record.id), record)) {
			fingerprints[record.id] = await recordFingerprint(record);
			cached[record.id] = version;
		} else {
			// The upload was acknowledged, but a later local edit is still pending.
			if (submitted) fingerprints[record.id] = await recordFingerprint(submitted);
			delete cached[record.id];
		}
	};
	const keepServer = async (conflict: SyncConflict) => {
		const expected = singleRecord(conflict.collection, conflict.local);
		if (conflict.server) {
			await apply(singleRecord(conflict.collection, conflict.server), expected);
			await rememberServer(conflict.collection, conflict.server, conflict.serverVersion, conflict.local);
		} else {
			await apply(emptySyncRecords(), expected, { [conflict.collection]: [conflict.id] });
			delete checkpoint.fingerprints[conflict.collection]?.[conflict.id];
			delete checkpoint.versions![conflict.collection]?.[conflict.id];
			(checkpoint.baseVersions![conflict.collection] ??= {})[conflict.id] = 'missing';
		}
		await saveSyncCheckpoint(checkpoint);
	};
	const choose = async (conflict: SyncConflict): Promise<SyncResolution | 'changed'> => {
		ensureUser();
		const choice = await onConflict(conflict);
		ensureUser();
		if (choice === 'later') throw new SyncConflictDeferredError();
		if (!sameSyncRecord(currentRecord(conflict.collection, conflict.id), conflict.local)) return 'changed';
		return choice;
	};

	const upload = async (records: SyncRecords, overrides: SyncVersions = {}): Promise<void> => {
		const bases: SyncVersions = structuredClone(baseVersions());
		for (const collection of SYNC_COLLECTIONS) Object.assign(bases[collection] ??= {}, overrides[collection]);
		for (const batch of buildSyncBatches({ ...state.settings, activeGroupId: resolvedGroupId }, records, bases)) {
			onProgress(`Uploading changes (batch ${++batchNumber})...`);
			const result = await syncWithBackend(batch, userId);
			requireSyncProtocol(result);
			const serverGroupId = result.settings?.activeGroupId || resolvedGroupId;
			if (serverGroupId !== resolvedGroupId) {
				resolvedGroupId = serverGroupId;
				checkpoint = await getSyncCheckpoint(`${userId}:${resolvedGroupId}`);
				checkpoint.baseVersions ??= structuredClone(checkpoint.versions ?? {});
				checkpoint.versions ??= {};
				return;
			}
			const accepted = responseRecords(result);
			const conflicts = result.conflicts ?? [];
			for (const collection of SYNC_COLLECTIONS) for (const row of batch[collection]) {
				if (!result.acceptedVersions?.[collection]?.[row.id] && !conflicts.some(item => item.collection === collection && item.id === row.id)) {
					throw new Error('The server did not acknowledge a sync record. Please retry.');
				}
			}
			await apply(accepted, batch);
			for (const collection of SYNC_COLLECTIONS) for (const row of accepted[collection]) {
				const version = result.acceptedVersions?.[collection]?.[row.id];
				if (!version) throw new Error('The server omitted an accepted record version.');
				await rememberServer(collection, row, version, batch[collection].find(item => item.id === row.id));
			}
			await saveSyncCheckpoint(checkpoint);
			for (const item of conflicts) {
				const local = currentRecord(item.collection, item.id);
				if (!local) continue;
				const conflict = { ...item, local };
				const choice = await choose(conflict);
				if (choice === 'server') await keepServer(conflict);
				else {
					const current = currentRecord(item.collection, item.id);
					if (current) await upload(singleRecord(item.collection, current), choice === 'local'
						? { [item.collection]: { [item.id]: item.serverVersion } } : {});
				}
			}
		}
	};

	const pending = await pendingSyncRecords(scoped(), checkpoint);
	// Suggestions and usage counts are derived on the server from accepted entries.
	pending.merchants = [];
	await upload(pending);

	for (const collection of SYNC_COLLECTIONS) {
		let cursor = '';
		let received = 0;
		const seen = new Set<string>();
		const localRecords = scoped()[collection].slice().sort((a,b) => a.id.localeCompare(b.id));
		do {
			onProgress(`Checking ${collection} (${received} records)...`);
			const known = Object.fromEntries(localRecords.filter(row => row.id > cursor && checkpoint.versions?.[collection]?.[row.id]).slice(0,100)
				.map(row => [row.id, checkpoint.versions![collection]![row.id]]));
			const result = await syncWithBackend({ settings: { ...state.settings, activeGroupId: resolvedGroupId }, ...emptySyncRecords(),
				sync: { mode: 'pull', version: 3, collection, cursor, limit: 100, known } }, userId);
			requireSyncProtocol(result);
			if (!result.page || result.page.collection !== collection) throw new Error('Invalid sync page returned by backend.');
			const serverGroupId = result.settings?.activeGroupId || resolvedGroupId;
			if (serverGroupId !== resolvedGroupId) {
				if (collection !== 'groups' || cursor) throw new Error('Your group changed during sync. Please retry.');
				resolvedGroupId = serverGroupId;
				checkpoint = await getSyncCheckpoint(`${userId}:${resolvedGroupId}`);
				checkpoint.baseVersions ??= structuredClone(checkpoint.versions ?? {});
				checkpoint.versions ??= {};
			}
			for (const id of Object.keys(result.page.versions)) seen.add(id);
			received += Object.keys(result.page.versions).length;
			const pageRecords = emptySyncRecords();
			const pageExpected = emptySyncRecords();
			for (const server of result[collection] ?? []) {
				const version = result.page.versions[server.id];
				const local = currentRecord(collection, server.id);
				const dirty = local && checkpoint.fingerprints[collection]?.[local.id] !== await recordFingerprint(local);
				if (collection !== 'merchants' && dirty) {
					if (baseVersions()[collection]?.[server.id] !== version) {
						const conflict = { collection, id: server.id, serverVersion: version, server, local };
						const choice = await choose(conflict);
						if (choice === 'server') await keepServer(conflict);
						else {
							const current = currentRecord(collection, server.id);
							if (current) await upload(singleRecord(collection, current), choice === 'local'
								? { [collection]: { [server.id]: version } } : {});
						}
					}
					continue;
				}
				(pageRecords[collection] as SyncRecord[]).push(server);
				if (local) (pageExpected[collection] as SyncRecord[]).push(local);
			}
			await apply(pageRecords, pageExpected);
			for (const server of pageRecords[collection]) await rememberServer(collection, server, result.page.versions[server.id]);
			await saveSyncCheckpoint(checkpoint);
			backendSettings = result.settings ?? backendSettings;
			syncedAt = result.syncedAt;
			const nextCursor = result.page.hasMore ? result.page.nextCursor : '';
			if (result.page.hasMore && (!nextCursor || nextCursor <= cursor)) throw new Error('Sync pagination did not advance.');
			cursor = nextCursor || '';
		} while (cursor);
		const removed: string[] = [];
		const removalExpected = emptySyncRecords();
		for (const local of scoped()[collection]) {
			if (seen.has(local.id) || (collection !== 'merchants' && !baseVersions()[collection]?.[local.id])) continue;
			const dirty = checkpoint.fingerprints[collection]?.[local.id] !== await recordFingerprint(local);
			const conflict = { collection, id: local.id, serverVersion: 'missing', server: null, local };
			if (collection !== 'merchants' && dirty) {
				const choice = await choose(conflict);
				if (choice !== 'server') {
					const current = currentRecord(collection, local.id);
					if (current) await upload(singleRecord(collection, current), choice === 'local' ? { [collection]: { [local.id]: 'missing' } } : {});
					continue;
				}
			}
			if (collection !== 'merchants' && dirty) await keepServer(conflict);
			else {
				removed.push(local.id);
				(removalExpected[collection] as SyncRecord[]).push(local);
			}
		}
		if (removed.length) {
			await apply(emptySyncRecords(), removalExpected, { [collection]: removed });
			for (const id of removed) {
				delete checkpoint.fingerprints[collection]?.[id];
				delete checkpoint.versions![collection]?.[id];
				(checkpoint.baseVersions![collection] ??= {})[id] = 'missing';
			}
			await saveSyncCheckpoint(checkpoint);
		}
	}
	ensureUser();
	return { ...latest(), settings: { ...latest().settings,
		activeGroupId: latest().settings.activeGroupId !== groupId ? latest().settings.activeGroupId : resolvedGroupId,
		baseCurrency: normalizeCurrencyCode(backendSettings.baseCurrency), lastSyncedAt: syncedAt || isoNow() } };
}

export function mergeById<T extends { id: string }>(local: T[], remote: T[]): T[] {
	const map = new Map(local.map((record) => [record.id, record]));
	for (const record of remote) map.set(record.id, record);
	return [...map.values()];
}

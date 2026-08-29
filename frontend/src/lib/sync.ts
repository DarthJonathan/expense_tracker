import { mergeRemoteState } from './db';
import { apiPath } from './api';
import { authFetch } from './auth';
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

interface SyncPayload {
	settings: FinanceState['settings'];
	groups: Group[];
	accounts: Account[];
	categories: Category[];
	entries: LedgerEntry[];
	adjustments: CategoryAdjustment[];
	merchants: Merchant[];
}

interface SyncResponse {
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

function newerOnly<T extends { id: string; updatedAt: string }>(
	local: T[],
	remote: T[],
	acceptCanonicalEqualTimestamp = false
): T[] {
	const localMap = new Map(local.map((record) => [record.id, record]));

	return remote.filter((remoteRecord) => {
		const localRecord = localMap.get(remoteRecord.id);
		if (!localRecord) return true;
		const remoteUpdatedAt = new Date(remoteRecord.updatedAt).getTime();
		const localUpdatedAt = new Date(localRecord.updatedAt).getTime();
		return (
			remoteUpdatedAt > localUpdatedAt ||
			(acceptCanonicalEqualTimestamp &&
				remoteUpdatedAt === localUpdatedAt &&
				JSON.stringify(remoteRecord) !== JSON.stringify(localRecord))
		);
	});
}

async function syncWithBackend(syncPayload: SyncPayload): Promise<SyncResponse> {
	const response = await authFetch(apiPath('/api/v1/sync'), {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(syncPayload)
	});

	if (!response.ok) {
		let message = `Backend sync failed (${response.status})`;
		try {
			const data = (await response.json()) as { error?: string };
			if (data?.error) message = data.error;
		} catch {
			// keep generic message
		}
		throw new Error(message);
	}

	const raw = (await response.json()) as SyncResponse | WrappedSyncResponse;
	if ('success' in raw) {
		if (!raw.success) {
			throw new Error(raw.error || 'Backend sync failed');
		}
		return raw.data ?? { groups: [], accounts: [], categories: [], entries: [], adjustments: [], merchants: [], syncedAt: '' };
	}

	return raw;
}

export async function syncFinanceState(
	state: FinanceState,
	getLatestState: () => FinanceState = () => state
): Promise<FinanceState> {
	const groupId = state.settings.activeGroupId;
	const inGroup = <T extends { groupId: string }>(records: T[]) => records.filter((record) => record.groupId === groupId);
	const payload: SyncPayload = {
		settings: state.settings,
		groups: state.groups.filter((group) => group.id === groupId),
		accounts: inGroup(state.accounts),
		categories: inGroup(state.categories),
		entries: inGroup(state.entries),
		adjustments: inGroup(state.adjustments),
		merchants: inGroup(state.merchants)
	};

	const backendData = await syncWithBackend(payload);
	const groups = backendData.groups ?? [];
	const accounts = backendData.accounts ?? [];
	const categories = backendData.categories ?? [];
	const entries = backendData.entries ?? [];
	const adjustments = backendData.adjustments ?? [];
	const merchants = backendData.merchants ?? [];
	const latestState = getLatestState();
	const remoteGroupIds = groups.map((group) => group.id);
	const resolvedActiveGroupId =
		latestState.settings.activeGroupId !== groupId
			? latestState.settings.activeGroupId
			: remoteGroupIds.length > 0 && !remoteGroupIds.includes(latestState.settings.activeGroupId)
			? remoteGroupIds[0]
			: latestState.settings.activeGroupId;

	const remote = {
		groups: newerOnly(latestState.groups, groups),
		accounts: newerOnly(latestState.accounts, accounts),
		categories: newerOnly(latestState.categories, categories),
		entries: newerOnly(latestState.entries, entries, true),
		adjustments: newerOnly(latestState.adjustments, adjustments),
		merchants: newerOnly(latestState.merchants, merchants)
	};

	await mergeRemoteState(remote);

	return {
		...latestState,
		settings: {
			...latestState.settings,
			activeGroupId: resolvedActiveGroupId,
			baseCurrency: normalizeCurrencyCode(backendData.settings?.baseCurrency ?? latestState.settings.baseCurrency),
			lastSyncedAt: backendData.syncedAt || isoNow()
		},
		groups: mergeById(latestState.groups, remote.groups),
		accounts: mergeById(latestState.accounts, remote.accounts),
		categories: mergeById(latestState.categories, remote.categories),
		entries: mergeById(latestState.entries, remote.entries),
		adjustments: mergeById(latestState.adjustments, remote.adjustments),
		merchants: mergeById(latestState.merchants, remote.merchants)
	};
}

export function mergeById<T extends { id: string }>(local: T[], remote: T[]): T[] {
	const map = new Map(local.map((record) => [record.id, record]));
	for (const record of remote) map.set(record.id, record);
	return [...map.values()];
}

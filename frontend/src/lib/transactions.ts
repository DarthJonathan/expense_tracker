import { apiPath } from './api';
import { authFetch } from './auth';
import type { EntryType, LedgerEntry } from './types';

interface ExpenseListPayload {
	success: boolean;
	error?: string;
	data?: LedgerEntry[];
}

export async function searchTransactionsRemote(params: {
	groupId: string;
	query?: string;
	monthsBack?: number | null;
	type?: EntryType;
	limit?: number;
}): Promise<LedgerEntry[]> {
	const searchParams = new URLSearchParams();
	const trimmedQuery = params.query?.trim() ?? '';
	if (trimmedQuery) searchParams.set('q', trimmedQuery);
	if (params.monthsBack && params.monthsBack > 0) searchParams.set('monthsBack', String(params.monthsBack));
	if (params.type) searchParams.set('type', params.type);
	if (params.limit && params.limit > 0) searchParams.set('limit', String(params.limit));

	const suffix = searchParams.size > 0 ? `?${searchParams.toString()}` : '';
	const response = await authFetch(apiPath(`/api/v1/groups/${params.groupId}/transactions${suffix}`), {
		method: 'GET'
	});

	const payload = (await response.json()) as ExpenseListPayload;
	if (!response.ok || !payload.success) {
		throw new Error(payload.error || `Search failed (${response.status})`);
	}

	return payload.data ?? [];
}

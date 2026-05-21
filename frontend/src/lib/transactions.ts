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

interface ExpensePayload {
	success: boolean;
	error?: string;
	data?: LedgerEntry;
}

export async function updateTransactionRemote(params: {
	groupId: string;
	transactionId: string;
	accountId: string;
	categoryId: string;
	type: EntryType;
	amount: number;
	currency: string;
	occurredOn: string;
	merchant: string;
	note: string;
}): Promise<LedgerEntry> {
	const response = await authFetch(
		apiPath(`/api/v1/groups/${params.groupId}/transactions/${params.transactionId}`),
		{
			method: 'PUT',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				accountId: params.accountId,
				categoryId: params.categoryId,
				type: params.type,
				amount: params.amount,
				currency: params.currency,
				occurredOn: params.occurredOn,
				merchant: params.merchant,
				note: params.note
			})
		}
	);

	const payload = (await response.json()) as ExpensePayload;
	if (!response.ok || !payload.success || !payload.data) {
		throw new Error(payload.error || `Update failed (${response.status})`);
	}

	return payload.data;
}

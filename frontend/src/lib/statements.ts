import { apiPath } from './api';
import { authFetch } from './auth';
import type { ParsedStatement } from './statement-parser';
import type {
	StatementIngestion,
	StatementIngestionDetail,
	StatementIngestionRow,
	StatementReviewStatus
} from './types';

export { extractEmbeddedPdfText, parseDbsStatementText } from './statement-parser';
export type { ParsedStatement, ParsedStatementKind, ParsedStatementRow } from './statement-parser';

interface ApiPayload<T> {
	success: boolean;
	error?: string;
	data?: T;
}

async function request<T>(path: string, init: RequestInit, recoverMissingData?: () => T | Promise<T>): Promise<T> {
	const response = await authFetch(apiPath(path), init);
	const text = await response.text();
	let decoded: unknown;
	try {
		decoded = text ? JSON.parse(text) : null;
	} catch {
		throw new Error(`Statement API returned an invalid response (${response.status})`);
	}
	const payload = decoded as Partial<ApiPayload<T>> | null;
	if (!response.ok) {
		throw new Error(payload?.error || `Statement request failed (${response.status})`);
	}
	if (payload && typeof payload.success === 'boolean') {
		if (!payload.success) throw new Error(payload.error || `Statement request failed (${response.status})`);
		if (payload.data !== undefined && payload.data !== null) return payload.data;
		if (recoverMissingData) return recoverMissingData();
		throw new Error(`Statement API returned success without data for ${init.method ?? 'GET'} ${path}`);
	}
	// Some same-origin proxies unwrap the standard { success, data } envelope.
	// Accept only recognizable statement payloads so an unrelated 200 response
	// cannot be mistaken for financial data.
	if (Array.isArray(decoded) || (isRecord(decoded) && isRecord(decoded.ingestion) && Array.isArray(decoded.rows))) return decoded as T;
	throw new Error(`Statement API returned an unexpected response for ${init.method ?? 'GET'} ${path}`);
}

function isRecord(value: unknown): value is Record<string, unknown> { return typeof value === 'object' && value !== null; }

export function listStatementIngestions(groupId: string): Promise<StatementIngestion[]> {
	return request(`/api/v1/groups/${groupId}/statement-ingestions`, { method: 'GET' }, () => []);
}

export function getStatementIngestion(groupId: string, ingestionId: string): Promise<StatementIngestionDetail> {
	return request(`/api/v1/groups/${groupId}/statement-ingestions/${ingestionId}`, { method: 'GET' });
}

export function createStatementIngestion(groupId: string, statement: ParsedStatement): Promise<StatementIngestionDetail> {
	// This payload is intentionally constructed from normalized values only. Do
	// not add File, Blob, OCR text, PDF bytes, or a data URL here.
	return request(`/api/v1/groups/${groupId}/statement-ingestions`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(statement)
	});
}

type StatementRowPatch = Partial<
	Pick<
		StatementIngestionRow,
		| 'occurredOn'
		| 'merchant'
		| 'amount'
		| 'currency'
		| 'foreignAmount'
		| 'foreignCurrency'
		| 'statementKind'
		| 'type'
		| 'accountId'
		| 'categoryId'
		| 'note'
		| 'matchExpenseId'
		| 'warningCodes'
	>
> & { reviewStatus?: StatementReviewStatus };

export function updateStatementRow(
	groupId: string,
	ingestionId: string,
	rowId: string,
	patch: StatementRowPatch
): Promise<StatementIngestionDetail> {
	return request(`/api/v1/groups/${groupId}/statement-ingestions/${ingestionId}/rows/${rowId}`, {
		method: 'PATCH',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(patch)
	});
}

export async function deleteStatementRow(groupId: string, ingestionId: string, rowId: string): Promise<StatementIngestionDetail> {
	const detail = await request(`/api/v1/groups/${groupId}/statement-ingestions/${ingestionId}/rows/${rowId}`, { method: 'DELETE' }, () => getStatementIngestion(groupId, ingestionId));
	if (detail.rows.some((row) => row.id === rowId)) throw new Error('The server acknowledged removal, but the staging transaction is still present. Refresh and retry.');
	return detail;
}

export function createCombinedStatementMatch(
	groupId: string,
	ingestionId: string,
	rowIds: string[],
	matchExpenseId: string
): Promise<StatementIngestionDetail> {
	return request(`/api/v1/groups/${groupId}/statement-ingestions/${ingestionId}/combined-matches`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ rowIds, matchExpenseId })
	});
}

export function createCombinedStatementNew(groupId: string, ingestionId: string, rowIds: string[], newTransaction: import('$lib/types').CombinedStatementTransaction): Promise<StatementIngestionDetail> {
	return request(`/api/v1/groups/${groupId}/statement-ingestions/${ingestionId}/combined-matches`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify({ rowIds, newTransaction })
	});
}

export function dissolveCombinedStatementMatch(groupId: string, ingestionId: string, combinedMatchId: string): Promise<StatementIngestionDetail> {
	return request(`/api/v1/groups/${groupId}/statement-ingestions/${ingestionId}/combined-matches/${combinedMatchId}`, { method: 'DELETE' });
}

export function confirmStatementIngestion(groupId: string, ingestionId: string): Promise<StatementIngestionDetail> {
	return request(`/api/v1/groups/${groupId}/statement-ingestions/${ingestionId}/confirm`, { method: 'POST' });
}

export function deleteStatementIngestion(groupId: string, ingestionId: string): Promise<StatementIngestionDetail> {
	return request(`/api/v1/groups/${groupId}/statement-ingestions/${ingestionId}`, { method: 'DELETE' });
}

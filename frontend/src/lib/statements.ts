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

async function request<T>(path: string, init: RequestInit): Promise<T> {
	const response = await authFetch(apiPath(path), init);
	const payload = (await response.json()) as ApiPayload<T>;
	if (!response.ok || !payload.success || !payload.data) {
		throw new Error(payload.error || `Statement request failed (${response.status})`);
	}
	return payload.data;
}

export function listStatementIngestions(groupId: string): Promise<StatementIngestion[]> {
	return request(`/api/v1/groups/${groupId}/statement-ingestions`, { method: 'GET' });
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

export function confirmStatementIngestion(groupId: string, ingestionId: string): Promise<StatementIngestionDetail> {
	return request(`/api/v1/groups/${groupId}/statement-ingestions/${ingestionId}/confirm`, { method: 'POST' });
}

export function deleteStatementIngestion(groupId: string, ingestionId: string): Promise<StatementIngestionDetail> {
	return request(`/api/v1/groups/${groupId}/statement-ingestions/${ingestionId}`, { method: 'DELETE' });
}

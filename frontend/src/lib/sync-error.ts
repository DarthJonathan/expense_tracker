import { SYNC_COLLECTIONS, type SyncCollection } from './sync-protocol';

export interface InaccessibleSyncRecord {
	collection: SyncCollection;
	id: string;
}

export class SyncHTTPError extends Error {
	constructor(
		readonly status: number,
		detail?: string,
		readonly requestId?: string | null,
		readonly inaccessibleRecord?: InaccessibleSyncRecord
	) {
		super(syncErrorMessage(status, detail, requestId));
		this.name = 'SyncHTTPError';
	}
}

function inaccessibleRecord(value: unknown): InaccessibleSyncRecord | undefined {
	if (!value || typeof value !== 'object') return;
	const { collection, id } = value as Record<string, unknown>;
	if (typeof collection !== 'string' || !SYNC_COLLECTIONS.includes(collection as SyncCollection) || typeof id !== 'string' ||
		!/^[\da-f]{8}-[\da-f]{4}-[\da-f]{4}-[\da-f]{4}-[\da-f]{12}$/i.test(id)) return;
	return { collection: collection as SyncCollection, id };
}

export function syncErrorMessage(status: number, detail?: string, requestId?: string | null): string {
	const message = detail?.trim() || 'The server returned no error details. Please try again.';
	return `Sync failed (HTTP ${status}): ${message}${requestId ? ` Request ID: ${requestId}` : ''}`;
}

export async function readSyncError(response: Response): Promise<Error> {
	let detail: string | undefined;
	let denied: InaccessibleSyncRecord | undefined;
	let requestId = response.headers.get('x-request-id');
	try {
		const text = await response.text();
		try {
			const data = JSON.parse(text) as { error?: unknown; message?: unknown; requestId?: unknown; inaccessibleRecord?: unknown } | null;
			const message = data?.error || data?.message;
			if (typeof message === 'string') detail = message;
			if (!requestId && typeof data?.requestId === 'string') requestId = data.requestId;
			denied = inaccessibleRecord(data?.inaccessibleRecord);
		} catch {
			// Gateways may return plain text; do not display their HTML error pages.
			if (!text.trimStart().startsWith('<')) detail = text.trim().slice(0, 500);
		}
	} catch {
		// Retain the HTTP status and request ID even if reading the body fails.
	}
	// Older backends already identify the submitted UUID in their diagnostic text.
	// Support those responses while deploying the structured error metadata.
	if (response.status === 403 && !denied) {
		const match = detail?.match(/^sync record is not accessible \((\w+) record ([\da-f-]+)\)$/i);
		if (match) denied = inaccessibleRecord({ collection: match[1].toLowerCase(), id: match[2] });
	}
	return new SyncHTTPError(response.status, detail, requestId, denied);
}

export function syncErrorMessage(status: number, detail?: string, requestId?: string | null): string {
	const message = detail?.trim() || 'The server returned no error details. Please try again.';
	return `Sync failed (HTTP ${status}): ${message}${requestId ? ` Request ID: ${requestId}` : ''}`;
}

export async function readSyncError(response: Response): Promise<Error> {
	let detail: string | undefined;
	let requestId = response.headers.get('x-request-id');
	try {
		const text = await response.text();
		try {
			const data = JSON.parse(text) as { error?: unknown; message?: unknown; requestId?: unknown } | null;
			const message = data?.error || data?.message;
			if (typeof message === 'string') detail = message;
			if (!requestId && typeof data?.requestId === 'string') requestId = data.requestId;
		} catch {
			// Gateways may return plain text; do not display their HTML error pages.
			if (!text.trimStart().startsWith('<')) detail = text.trim().slice(0, 500);
		}
	} catch {
		// Retain the HTTP status and request ID even if reading the body fails.
	}
	return new Error(syncErrorMessage(response.status, detail, requestId));
}

import { randomUUID } from 'node:crypto';

const HOP_BY_HOP_HEADERS = [
	'connection', 'keep-alive', 'proxy-authenticate', 'proxy-authorization',
	'te', 'trailer', 'trailers', 'transfer-encoding', 'upgrade', 'host', 'content-length'
];

export async function proxyApiRequest(request: Request, path: string, backendBaseUrl: string): Promise<Response> {
	const url = new URL(request.url);
	const target = `${backendBaseUrl}/api/v1/${path}${url.search}`;
	const headers = new Headers(request.headers);
	for (const header of HOP_BY_HOP_HEADERS) headers.delete(header);
	const requestId = randomUUID();
	headers.set('x-request-id', requestId);
	const body = !['GET', 'HEAD'].includes(request.method) ? await request.arrayBuffer() : undefined;
	const startedAt = Date.now();
	// Keep credentials, query parameters and financial records out of proxy logs.
	const context = { requestId, method: request.method, path: url.pathname, bodyBytes: body?.byteLength ?? 0 };

	let upstream: Response;
	try {
		upstream = await fetch(target, { method: request.method, headers, body });
	} catch (error) {
		const cause = error instanceof Error ? (error as Error & { cause?: { code?: string } }).cause : undefined;
		console.error('[api proxy] Backend connection failed', {
			...context, status: 502, durationMs: Date.now() - startedAt, code: cause?.code
		});
		return Response.json({
			success: false,
			error: 'The server could not reach the backend. Please try again.',
			requestId
		}, { status: 502, headers: { 'x-request-id': requestId, 'cache-control': 'no-store' } });
	}

	const responseHeaders = new Headers(upstream.headers);
	for (const header of HOP_BY_HOP_HEADERS) responseHeaders.delete(header);
	responseHeaders.set('x-request-id', upstream.headers.get('x-request-id') || requestId);
	if (!upstream.ok) {
		responseHeaders.set('cache-control', 'no-store');
		console.error('[api proxy] Backend request failed', {
			...context, requestId: responseHeaders.get('x-request-id'),
			status: upstream.status, durationMs: Date.now() - startedAt
		});
	}
	return new Response(upstream.body, {
		status: upstream.status, statusText: upstream.statusText, headers: responseHeaders
	});
}

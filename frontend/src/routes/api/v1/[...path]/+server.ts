import { env } from '$env/dynamic/private';
import type { RequestHandler } from './$types';

const HOP_BY_HOP_HEADERS = new Set([
	'connection',
	'keep-alive',
	'proxy-authenticate',
	'proxy-authorization',
	'te',
	'trailers',
	'transfer-encoding',
	'upgrade',
	'host',
	'content-length'
]);

function trimTrailingSlash(value: string): string {
	return value.replace(/\/+$/, '');
}

function getBackendBaseUrl(): string {
	const configured =
		env.BACKEND_API_URL?.trim() || env.API_BASE_URL?.trim() || env.VITE_API_BASE_URL?.trim();
	return configured ? trimTrailingSlash(configured) : 'http://backend:8080';
}

async function proxy(request: Request, path: string, method: string): Promise<Response> {
	const url = new URL(request.url);
	const target = `${getBackendBaseUrl()}/api/v1/${path}${url.search}`;

	const headers = new Headers(request.headers);
	for (const header of HOP_BY_HOP_HEADERS) {
		headers.delete(header);
	}

	const hasBody = !['GET', 'HEAD'].includes(method);
	const body = hasBody ? await request.arrayBuffer() : undefined;

	const upstream = await fetch(target, {
		method,
		headers,
		body
	});

	const responseHeaders = new Headers(upstream.headers);
	for (const header of HOP_BY_HOP_HEADERS) {
		responseHeaders.delete(header);
	}

	return new Response(upstream.body, {
		status: upstream.status,
		statusText: upstream.statusText,
		headers: responseHeaders
	});
}

const handle: RequestHandler = async ({ request, params }) => {
	return proxy(request, params.path ?? '', request.method);
};

export const GET = handle;
export const POST = handle;
export const PUT = handle;
export const PATCH = handle;
export const DELETE = handle;
export const OPTIONS = handle;


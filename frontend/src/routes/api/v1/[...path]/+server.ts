import { env } from '$env/dynamic/private';
import type { RequestHandler } from './$types';
import { proxyApiRequest } from '$lib/server/api-proxy';

function trimTrailingSlash(value: string): string {
	return value.replace(/\/+$/, '');
}

function getBackendBaseUrl(): string {
	const configured =
		env.BACKEND_API_URL?.trim() || env.API_BASE_URL?.trim() || env.VITE_API_BASE_URL?.trim();
	return configured ? trimTrailingSlash(configured) : 'http://backend:8080';
}

const handle: RequestHandler = async ({ request, params }) => {
	return proxyApiRequest(request, params.path ?? '', getBackendBaseUrl());
};

export const GET = handle;
export const POST = handle;
export const PUT = handle;
export const PATCH = handle;
export const DELETE = handle;
export const OPTIONS = handle;


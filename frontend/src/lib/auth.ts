import { apiPath, getApiBaseUrl } from './api';

export interface AuthUser {
	id: string;
	email: string;
	displayName: string;
}

export interface AuthSession {
	token: string;
	expiresAt: string;
	user: AuthUser;
}

export interface APIKeyData {
	id: string;
	name: string;
	key: string;
	keyPrefix: string;
	createdAt: string;
	expiresAt?: string;
}

interface AuthPayload {
	success: boolean;
	error?: string;
	data?: AuthSession;
}

interface APIKeyPayload {
	success: boolean;
	error?: string;
	data?: APIKeyData;
}

const STORAGE_KEY = 'expense-auth-session';

export function getStoredSession(): AuthSession | null {
	try {
		const raw = localStorage.getItem(STORAGE_KEY);
		if (!raw) return null;
		const parsed = JSON.parse(raw) as AuthSession;
		if (!parsed?.token || !parsed?.user?.id || !parsed?.expiresAt) return null;
		if (new Date(parsed.expiresAt).getTime() <= Date.now()) {
			localStorage.removeItem(STORAGE_KEY);
			return null;
		}
		return parsed;
	} catch {
		return null;
	}
}

export function storeSession(session: AuthSession): void {
	localStorage.setItem(STORAGE_KEY, JSON.stringify(session));
}

export function clearSession(): void {
	localStorage.removeItem(STORAGE_KEY);
}

export async function register(
	email: string,
	password: string,
	displayName: string,
	inviteCode: string
): Promise<AuthSession> {
	return authRequest('/api/v1/auth/register', { email, password, displayName, inviteCode });
}

export async function login(email: string, password: string): Promise<AuthSession> {
	return authRequest('/api/v1/auth/login', { email, password });
}

export async function loginWithApiKey(apiKey: string): Promise<AuthSession> {
	return authRequest('/api/v1/auth/api-keys/authenticate', { apiKey });
}

export async function generateApiKey(name: string): Promise<APIKeyData> {
	const response = await authFetch(apiPath('/api/v1/auth/api-keys'), {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify({ name })
	});

	const raw = (await parseJsonOrThrow<APIKeyPayload>(response, 'Failed to generate API key')) as APIKeyPayload;
	if (!response.ok || !raw.success || !raw.data) {
		throw new Error(raw.error || 'Failed to generate API key');
	}

	return raw.data;
}

export async function authFetch(input: string, init?: RequestInit): Promise<Response> {
	const session = getStoredSession();
	if (!session?.token) throw new Error('Not authenticated');

	const headers = new Headers(init?.headers ?? {});
	headers.set('Authorization', `Bearer ${session.token}`);

	return safeFetch(input, { ...init, headers });
}

async function authRequest(path: string, payload: Record<string, string>): Promise<AuthSession> {
	const url = apiPath(path);
	const response = await safeFetch(url, {
		method: 'POST',
		headers: { 'content-type': 'application/json' },
		body: JSON.stringify(payload)
	});
	const raw = (await parseJsonOrThrow<AuthPayload>(response, 'Authentication failed')) as AuthPayload;
	if (!response.ok || !raw.success || !raw.data) {
		throw new Error(raw.error || 'Authentication failed');
	}

	storeSession(raw.data);
	return raw.data;
}

async function safeFetch(input: string, init?: RequestInit): Promise<Response> {
	try {
		return await fetch(input, init);
	} catch {
		const base = getApiBaseUrl() || window.location.origin;
		throw new Error(`Cannot reach backend at ${base}. Start backend or update API base URL.`);
	}
}

async function parseJsonOrThrow<T>(response: Response, fallbackMessage: string): Promise<T> {
	try {
		return (await response.json()) as T;
	} catch {
		if (!response.ok) {
			throw new Error(`${fallbackMessage} (HTTP ${response.status})`);
		}
		throw new Error(fallbackMessage);
	}
}

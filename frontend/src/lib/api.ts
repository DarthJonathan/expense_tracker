function trimTrailingSlash(value: string): string {
	return value.replace(/\/+$/, '');
}

export function getApiBaseUrl(): string {
	const runtimeConfig =
		typeof window !== 'undefined' ? window.__APP_CONFIG__?.API_BASE_URL?.trim() : undefined;
	if (runtimeConfig) {
		return trimTrailingSlash(runtimeConfig);
	}

	const fromEnv = (import.meta.env.VITE_API_BASE_URL as string | undefined)?.trim();
	return fromEnv ? trimTrailingSlash(fromEnv) : '';
}

export function apiPath(path: string): string {
	const baseUrl = getApiBaseUrl();
	return baseUrl ? `${baseUrl}${path}` : path;
}

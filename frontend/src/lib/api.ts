export function getApiBaseUrl(): string {
	// BFF mode: browser calls same-origin /api/v1/*; backend target stays private server-side.
	return '';
}

export function apiPath(path: string): string {
	return path;
}

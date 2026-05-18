export function makeId(): string {
	if (globalThis.crypto?.randomUUID) {
		return globalThis.crypto.randomUUID();
	}

	return `${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

export function isoNow(): string {
	return new Date().toISOString();
}

export function todayInputValue(): string {
	const date = new Date();
	const offset = date.getTimezoneOffset();
	return new Date(date.getTime() - offset * 60_000).toISOString().slice(0, 10);
}

export function cents(value: FormDataEntryValue | number | string | null | undefined): number {
	const normalized = String(value ?? '')
		.trim()
		.replace(/[$,\s]/g, '');
	const parsed = Number(normalized || 0);
	return Number.isFinite(parsed) ? Math.round(parsed * 100) : 0;
}

export function currency(amountInCents: number): string {
	return new Intl.NumberFormat(undefined, {
		style: 'currency',
		currency: 'USD'
	}).format(amountInCents / 100);
}

export function formatSignedCurrency(amountInCents: number): string {
	const value = currency(Math.abs(amountInCents));
	return amountInCents < 0 ? `-${value}` : value;
}

export function normalizeText(value: FormDataEntryValue | null, fallback = ''): string {
	const text = String(value ?? '').trim();
	return text || fallback;
}

export function isActive<T extends { deletedAt?: string | null }>(record: T): boolean {
	return !record.deletedAt;
}

export function byUpdatedAt<T extends { updatedAt: string }>(a: T, b: T): number {
	return new Date(a.updatedAt).getTime() - new Date(b.updatedAt).getTime();
}

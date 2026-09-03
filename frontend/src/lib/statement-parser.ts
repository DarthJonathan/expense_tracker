export type ParsedStatementKind = 'transaction' | 'payment' | 'fee' | 'refund' | 'other';

export interface ParsedStatementRow {
	sourceRowKey: string;
	occurredOn?: string;
	merchant: string;
	amount: number;
	currency?: string;
	foreignAmount?: number;
	foreignCurrency?: string;
	statementKind?: ParsedStatementKind;
	type?: 'expense' | 'income';
	cardholder?: string;
	statementReference?: string;
	accountId?: string;
	categoryId?: string;
	note?: string;
	warningCodes?: string[];
}

export interface ParsedStatement {
	accountId: string;
	clientRequestId?: string;
	sourceFingerprint?: string;
	sourceName: string;
	institution?: string;
	statementCurrency: string;
	statementDate?: string;
	paymentDueDate?: string;
	periodStart?: string;
	periodEnd?: string;
	previousBalance?: number;
	declaredNewTransactionsTotal?: number;
	statementGrandTotal?: number;
	warnings?: string[];
	cardholders?: Array<{ name: string; declaredRowCount?: number; declaredSubtotal?: number }>;
	rows: ParsedStatementRow[];
}

interface PositionedText {
	str: string;
	x: number;
	y: number;
}

interface TextLine {
	page: number;
	line: number;
	text: string;
}

const monthNumbers: Record<string, number> = {
	JAN: 1,
	FEB: 2,
	MAR: 3,
	APR: 4,
	MAY: 5,
	JUN: 6,
	JUL: 7,
	AUG: 8,
	SEP: 9,
	OCT: 10,
	NOV: 11,
	DEC: 12
};

/**
 * Extracts a PDF text layer with the bundled PDF.js worker. The Blob is passed
 * directly as bytes; PDF.js does not receive a URL and cannot upload the file.
 */
export async function extractEmbeddedPdfText(file: Blob): Promise<string> {
	const promiseConstructor = Promise as unknown as {
		withResolvers?: () => { promise: Promise<unknown>; resolve: (value: unknown) => void; reject: (reason?: unknown) => void };
		try?: (callback: (...args: any[]) => any, ...args: any[]) => Promise<any>;
	};
	if (!promiseConstructor.withResolvers) {
		promiseConstructor.withResolvers = () => {
			let resolve!: (value: unknown) => void;
			let reject!: (reason?: unknown) => void;
			const promise = new Promise<unknown>((resolvePromise, rejectPromise) => {
				resolve = resolvePromise;
				reject = rejectPromise;
			});
			return { promise, resolve, reject };
		};
	}
	if (!promiseConstructor.try) {
		promiseConstructor.try = (callback, ...args) => Promise.resolve().then(() => callback(...args));
	}
	const { getDocument } = await import('pdfjs-dist/webpack.mjs');
	const data = new Uint8Array(await file.arrayBuffer());
	const loadingTask = getDocument({ data, useWorkerFetch: false, isEvalSupported: false });
	const document = await loadingTask.promise;
	try {
		const pages: string[] = [];
		for (let pageNumber = 1; pageNumber <= document.numPages; pageNumber += 1) {
			const page = await document.getPage(pageNumber);
			const content = await page.getTextContent();
			const items: PositionedText[] = [];
			for (const item of content.items) {
				if (!('str' in item) || !item.str.trim() || !('transform' in item)) continue;
				items.push({ str: item.str.trim(), x: item.transform[4], y: item.transform[5] });
			}
			const lines = positionItemsIntoLines(items);
			pages.push(`===== PAGE ${pageNumber} =====\n${lines.join('\n')}`);
			page.cleanup();
		}
		const text = pages.join('\n');
		if (text.replace(/===== PAGE \d+ =====/g, '').trim().length > 120) return text;
		return runLocalOcrAdapter(file);
	} finally {
		await document.destroy();
	}
}

function positionItemsIntoLines(items: PositionedText[]): string[] {
	const groups: Array<{ y: number; items: PositionedText[] }> = [];
	for (const item of items.sort((left, right) => right.y - left.y || left.x - right.x)) {
		let group = groups.find((candidate) => Math.abs(candidate.y - item.y) <= 1.5);
		if (!group) {
			group = { y: item.y, items: [] };
			groups.push(group);
		}
		group.items.push(item);
	}
	return groups
		.sort((left, right) => right.y - left.y)
		.map((group) => group.items.sort((left, right) => left.x - right.x).map((item) => item.str).join(' ').replace(/\s+/g, ' ').trim())
		.filter(Boolean);
}

async function runLocalOcrAdapter(file: Blob): Promise<string> {
	// A deployment may register a bundled PDF.js + OCR worker here. The adapter
	// receives only an in-process Blob, never a URL. No remote OCR fallback exists.
	const localOcr = (globalThis as typeof globalThis & { __spenditLocalPdfOcr?: (input: Blob) => Promise<string> })
		.__spenditLocalPdfOcr;
	if (!localOcr) {
		throw new Error('No embedded text layer was found. This build needs an on-device OCR worker for scanned statements.');
	}
	const text = await localOcr(file);
	if (!text.trim()) throw new Error('On-device OCR could not read this statement.');
	return text;
}

export function parseDbsStatementText(text: string, accountId: string, sourceName: string): ParsedStatement {
	const lines = splitTextLines(text);
	const statementCurrency = detectStatementCurrency(lines);
	const statementDate = findLabelledDates(lines, /STATEMENT\s+DATE/i)[0];
	const paymentDueDate = findLabelledDates(lines, /PAYMENT\s+DUE\s+DATE/i)[1] ?? findLabelledDates(lines, /PAYMENT\s+DUE\s+DATE/i)[0];
	const previousBalance = moneyFromLine(lines, /PREVIOUS\s+BALANCE/i);
	const statementGrandTotal = moneyFromLine(lines, /GRAND\s+TOTAL\s+FOR\s+ALL\s+CARD\s+ACCOUNTS/i) ?? moneyFromLine(lines, /^\s*TOTAL\s*:/i);
	const rows: ParsedStatementRow[] = [];
	const warnings = new Set<string>();
	const cardholderControls = new Map<string, { name: string; declaredSubtotal?: number }>();
	let currentCardholder = '';
	let withinNewTransactions = false;

	for (const line of lines) {
		const cardholderMatch = line.text.match(/^NEW\s+TRANSACTIONS\s+(.+)$/i);
		if (cardholderMatch) {
			currentCardholder = cardholderMatch[1].trim();
			withinNewTransactions = true;
			if (!cardholderControls.has(currentCardholder)) cardholderControls.set(currentCardholder, { name: currentCardholder });
			continue;
		}

		const subtotalMatch = line.text.match(/SUB[\s-]*TOTAL\s*:\s*([\d,]+\.\d{2})/i);
		if (subtotalMatch && currentCardholder) {
			cardholderControls.set(currentCardholder, {
				name: currentCardholder,
				declaredSubtotal: parseStatementCents(subtotalMatch[1])
			});
			continue;
		}

		const referenceMatch = line.text.match(/^REF(?:ERENCE)?\s+NO\s*:\s*(.+)$/i);
		if (referenceMatch && rows.length) {
			rows[rows.length - 1].statementReference = referenceMatch[1].trim().slice(0, 160);
			continue;
		}

		const foreignMatch = line.text.match(/^(YEN|RUPIAH|US\s+DOLLARS?|EURO|POUNDS?)\s+([\d,.]+)$/i);
		if (foreignMatch && rows.length) {
			const previous = rows[rows.length - 1];
			previous.foreignCurrency = foreignCurrencyCode(foreignMatch[1]);
			previous.foreignAmount = parseStatementCents(foreignMatch[2]);
			previous.merchant = previous.merchant.replace(/\s+(?:JP|ID|US)$/i, '').trim();
			previous.warningCodes = uniqueStrings([...(previous.warningCodes ?? []), 'foreign_currency']);
			continue;
		}

		const transaction = line.text.match(/^(\d{1,2})\s+([A-Z]{3})\s+(.+?)\s+([\d,]+\.\d{2})(?:\s+(CR|DR))?$/i);
		if (!transaction) {
			const transactionLike = line.text.match(/^(\d{1,2})\s+([A-Z]{3})\s+(?!\d{4}\b)(.+)$/i);
			if (transactionLike) {
				rows.push({
					sourceRowKey: `page-${line.page}-line-${line.line}`,
					occurredOn: normalizeStatementMonthDate(transactionLike[1], transactionLike[2], statementDate),
					merchant: transactionLike[3].trim(),
					amount: 0,
					currency: statementCurrency,
					statementKind: 'other',
					type: 'expense',
					cardholder: withinNewTransactions ? currentCardholder : '',
					statementReference: line.text.slice(0, 160),
					accountId,
					warningCodes: ['unreadable_row']
				});
			}
			continue;
		}
		const merchant = transaction[3].trim();
		const amount = parseStatementCents(transaction[4]);
		const occurredOn = normalizeStatementMonthDate(transaction[1], transaction[2], statementDate);
		const explicitCredit = transaction[5]?.toUpperCase() === 'CR';
		const payment = !withinNewTransactions && /(?:BILL\s+PAYMENT|PAYMENT)/i.test(merchant);
		const refund = !payment && (explicitCredit || /\b(?:REFUND|REVERSAL)\b/i.test(merchant));
		const statementKind: ParsedStatementKind = payment
			? 'payment'
			: refund
				? 'refund'
				: /\b(?:ANNUAL\s+FEE|FINANCE\s+CHARGE|LATE\s+FEE|GST\s*@)/i.test(merchant)
					? 'fee'
					: 'transaction';
		const warningCodes: string[] = [];
		if (!occurredOn || !merchant || amount <= 0) warningCodes.push('unreadable_row');
		rows.push({
			sourceRowKey: `page-${line.page}-line-${line.line}`,
			occurredOn,
			merchant,
			amount,
			currency: statementCurrency,
			statementKind,
			type: explicitCredit || payment || refund ? 'income' : 'expense',
			cardholder: withinNewTransactions ? currentCardholder : '',
			statementReference: line.text.slice(0, 160),
			accountId,
			warningCodes
		});
	}

	if (rows.length === 0) warnings.add('no_transaction_rows_detected');
	if (rows.some((row) => row.warningCodes?.includes('unreadable_row'))) warnings.add('unreadable_rows');
	if (rows.some((row) => row.warningCodes?.includes('foreign_currency'))) warnings.add('foreign_currency_rows');
	if (!statementDate) warnings.add('statement_date_not_detected');
	if (previousBalance == null) warnings.add('previous_balance_not_detected');
	if (statementGrandTotal == null) warnings.add('statement_total_not_detected');

	const controls = [...cardholderControls.values()];
	const declaredSubtotals = controls.map((item) => item.declaredSubtotal).filter((value): value is number => value != null);
	const dates = rows.map((row) => row.occurredOn).filter((value): value is string => Boolean(value)).sort();
	return {
		accountId,
		sourceName,
		institution: /\bDBS\b/i.test(text) ? 'DBS' : undefined,
		statementCurrency,
		statementDate,
		paymentDueDate,
		periodStart: dates[0],
		periodEnd: dates.at(-1),
		previousBalance: previousBalance ?? undefined,
		declaredNewTransactionsTotal: declaredSubtotals.length === controls.length && controls.length > 0
			? declaredSubtotals.reduce((total, amount) => total + amount, 0)
			: undefined,
		statementGrandTotal: statementGrandTotal ?? undefined,
		warnings: [...warnings],
		cardholders: controls,
		rows
	};
}

function splitTextLines(text: string): TextLine[] {
	const output: TextLine[] = [];
	let page = 1;
	let pageLine = 0;
	for (const raw of text.replace(/\u0000/g, ' ').replace(/\r/g, '\n').split('\n')) {
		const pageMatch = raw.match(/^===== PAGE (\d+) =====$/);
		if (pageMatch) {
			page = Number(pageMatch[1]);
			pageLine = 0;
			continue;
		}
		const normalized = raw.replace(/\s+/g, ' ').trim();
		if (!normalized) continue;
		pageLine += 1;
		output.push({ page, line: pageLine, text: normalized });
	}
	return output;
}

function detectStatementCurrency(lines: TextLine[]): string {
	const header = lines.find((line) => /AMOUNT\s*\((?:S\$|SGD)\)/i.test(line.text));
	if (header) return 'SGD';
	const explicit = lines.map((line) => line.text).join('\n').match(/STATEMENT\s+CURRENCY\s*[:\-]?\s*([A-Z]{3})/i)?.[1];
	return explicit?.toUpperCase() ?? 'SGD';
}

function findLabelledDates(lines: TextLine[], label: RegExp): string[] {
	const index = lines.findIndex((line) => label.test(line.text));
	if (index < 0) return [];
	const nearby = lines.slice(index, index + 4).map((line) => line.text).join(' ');
	return [...nearby.matchAll(/\b(\d{1,2})\s+([A-Z]{3})\s+(\d{4})\b/gi)]
		.map((match) => isoDate(Number(match[3]), monthNumbers[match[2].toUpperCase()], Number(match[1])))
		.filter((value): value is string => Boolean(value));
}

function moneyFromLine(lines: TextLine[], label: RegExp): number | null {
	for (const line of lines) {
		if (!label.test(line.text)) continue;
		const values = [...line.text.matchAll(/(?:S?\$\s*)?([\d,]+\.\d{2})(?:\s+CR)?/gi)];
		if (values.length) return parseStatementCents(values.at(-1)?.[1] ?? '');
	}
	return null;
}

function normalizeStatementMonthDate(dayValue: string, monthValue: string, statementDate?: string): string | undefined {
	if (!statementDate) return undefined;
	const statementYear = Number(statementDate.slice(0, 4));
	const statementMonth = Number(statementDate.slice(5, 7));
	const month = monthNumbers[monthValue.toUpperCase()];
	const day = Number(dayValue);
	if (!month || !day) return undefined;
	const year = month > statementMonth ? statementYear - 1 : statementYear;
	return isoDate(year, month, day);
}

function isoDate(year: number, month: number, day: number): string | undefined {
	if (!year || !month || !day || month > 12 || day > 31) return undefined;
	const date = new Date(Date.UTC(year, month - 1, day));
	if (date.getUTCFullYear() !== year || date.getUTCMonth() !== month - 1 || date.getUTCDate() !== day) return undefined;
	return `${year.toString().padStart(4, '0')}-${month.toString().padStart(2, '0')}-${day.toString().padStart(2, '0')}`;
}

function parseStatementCents(value: string): number {
	const normalized = value.replace(/[^\d.-]/g, '');
	if (!normalized) return 0;
	const amount = Math.round(Number(normalized) * 100);
	return Number.isFinite(amount) ? Math.abs(amount) : 0;
}

function foreignCurrencyCode(label: string): string {
	const normalized = label.replace(/\s+/g, ' ').trim().toUpperCase();
	if (normalized === 'YEN') return 'JPY';
	if (normalized === 'RUPIAH') return 'IDR';
	if (normalized.startsWith('US DOLLAR')) return 'USD';
	if (normalized === 'EURO') return 'EUR';
	if (normalized.startsWith('POUND')) return 'GBP';
	return normalized.slice(0, 3);
}

function uniqueStrings(values: string[]): string[] {
	return [...new Set(values)];
}

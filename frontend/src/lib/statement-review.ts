import type { Account, Category, LedgerEntry, StatementIngestionRow, StatementReviewStatus } from '$lib/types';

export function filterStatementRows(rows: StatementIngestionRow[], query: string, status: 'all' | StatementReviewStatus): StatementIngestionRow[] {
	const normalizedQuery = query.trim().toLowerCase();
	return rows.filter((row) => {
		const searchable = `${row.merchant} ${row.cardholder} ${row.statementReference}`.toLowerCase();
		return (!normalizedQuery || searchable.includes(normalizedQuery)) && (status === 'all' || row.reviewStatus === status);
	});
}

export function countStatementReviews(rows: StatementIngestionRow[]): Record<StatementReviewStatus, number> {
	const counts: Record<StatementReviewStatus, number> = { unreviewed: 0, new: 0, matched: 0, ignored: 0 };
	for (const row of rows) counts[row.reviewStatus]++;
	return counts;
}

export function matchedMasterUpdateCount(rows: StatementIngestionRow[]): number {
	return countStatementTransactions(rows, 'matched');
}

export function newMasterTransactionCount(rows: StatementIngestionRow[]): number {
	return countStatementTransactions(rows, 'new');
}

function countStatementTransactions(rows: StatementIngestionRow[], status: 'new' | 'matched'): number {
	const combinedGroups = new Set<string>();
	let ordinaryMatches = 0;
	for (const row of rows) {
		if (row.reviewStatus !== status) continue;
		if (row.combinedMatchId) combinedGroups.add(row.combinedMatchId);
		else ordinaryMatches++;
	}
	return ordinaryMatches + combinedGroups.size;
}

export function combinedStatementRowsCompatibilityError(rows: StatementIngestionRow[]): string | null {
	if (rows.length < 2) return 'Select at least two statement rows to combine.';
	const first = rows[0];
	if (!first.accountId || rows.some((row) => !row.accountId || row.currency !== first.currency || row.type !== first.type || row.accountId !== first.accountId)) {
		return 'Combined rows must use the same currency, type, and account.';
	}
	return null;
}

export function compatibleCombinedMatchEntries(
	selectedRows: StatementIngestionRow[],
	entries: LedgerEntry[],
	accounts: Account[],
	categories: Category[],
	query: string
): LedgerEntry[] {
	const first = selectedRows[0];
	if (!first?.accountId) return [];
	const normalizedQuery = query.trim().toLowerCase();
	return entries.filter((entry) => {
		if (entry.accountId !== first.accountId || entry.currency !== first.currency || entry.type !== first.type) return false;
		if (!normalizedQuery) return true;
		const account = accounts.find((item) => item.id === entry.accountId)?.name ?? '';
		const category = categories.find((item) => item.id === entry.categoryId)?.name ?? '';
		return `${entry.merchant} ${entry.occurredOn} ${entry.amount} ${(entry.amount / 100).toFixed(2)} ${account} ${category}`.toLowerCase().includes(normalizedQuery);
	});
}

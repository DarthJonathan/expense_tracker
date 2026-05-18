import type {
	Account,
	Category,
	CategoryAdjustment,
	LedgerEntry,
	PeriodCategoryTotal,
	PeriodGrain,
	PeriodSummary
} from './types';
import { isActive } from './utils';

export function periodKey(dateValue: string, grain: PeriodGrain): string {
	const date = new Date(`${dateValue}T00:00:00`);

	if (grain === 'day') {
		return dateValue;
	}

	if (grain === 'month') {
		return dateValue.slice(0, 7);
	}

	const day = date.getDay() || 7;
	const monday = new Date(date);
	monday.setDate(date.getDate() - day + 1);
	return monday.toISOString().slice(0, 10);
}

export function readablePeriod(key: string, grain: PeriodGrain): string {
	if (grain === 'month') {
		return new Intl.DateTimeFormat(undefined, { month: 'long', year: 'numeric' }).format(new Date(`${key}-01T00:00:00`));
	}

	if (grain === 'week') {
		return `Week of ${new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', year: 'numeric' }).format(new Date(`${key}T00:00:00`))}`;
	}

	return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', year: 'numeric' }).format(new Date(`${key}T00:00:00`));
}

export function getOpeningBalance(accounts: Account[]): number {
	return accounts.filter(isActive).reduce((sum, account) => sum + account.openingBalance, 0);
}

export function getCurrentBalance(accounts: Account[], entries: LedgerEntry[]): number {
	return (
		getOpeningBalance(accounts) +
		entries.filter(isActive).reduce((sum, entry) => sum + (entry.type === 'income' ? entry.amount : -entry.amount), 0)
	);
}

export function buildPeriodSummaries(
	accounts: Account[],
	categories: Category[],
	entries: LedgerEntry[],
	adjustments: CategoryAdjustment[],
	grain: PeriodGrain
): PeriodSummary[] {
	const categoryMap = new Map(categories.map((category) => [category.id, category]));
	const summaries = new Map<string, PeriodSummary>();
	const openingBalance = getOpeningBalance(accounts);
	const sortedEntries = entries.filter(isActive).sort((a, b) => a.occurredOn.localeCompare(b.occurredOn));
	const sortedAdjustments = adjustments.filter(isActive).sort((a, b) => a.occurredOn.localeCompare(b.occurredOn));

	function ensureSummary(key: string): PeriodSummary {
		let summary = summaries.get(key);
		if (!summary) {
			summary = {
				periodKey: key,
				income: 0,
				spent: 0,
				adjustments: 0,
				netCashFlow: 0,
				endingBalance: openingBalance,
				categories: []
			};
			summaries.set(key, summary);
		}

		return summary;
	}

	function ensureCategory(summary: PeriodSummary, categoryId: string): PeriodCategoryTotal {
		let total = summary.categories.find((item) => item.categoryId === categoryId);
		const category = categoryMap.get(categoryId);

		if (!total) {
			total = {
				periodKey: summary.periodKey,
				categoryId,
				categoryName: category?.name ?? 'Uncategorized',
				categoryColor: category?.color ?? '#64748b',
				spent: 0,
				income: 0,
				adjustments: 0,
				net: 0
			};
			summary.categories.push(total);
		}

		return total;
	}

	for (const entry of sortedEntries) {
		const summary = ensureSummary(periodKey(entry.occurredOn, grain));
		const categoryTotal = ensureCategory(summary, entry.categoryId);

		if (entry.type === 'expense') {
			summary.spent += entry.amount;
			categoryTotal.spent += entry.amount;
			categoryTotal.net -= entry.amount;
		} else {
			summary.income += entry.amount;
			categoryTotal.income += entry.amount;
			categoryTotal.net += entry.amount;
		}

		summary.netCashFlow = summary.income - summary.spent;
	}

	for (const adjustment of sortedAdjustments) {
		const summary = ensureSummary(periodKey(adjustment.occurredOn, grain));
		const categoryTotal = ensureCategory(summary, adjustment.categoryId);

		summary.adjustments += adjustment.amount;
		categoryTotal.adjustments += adjustment.amount;
		categoryTotal.net += adjustment.amount;
	}

	let rollingBalance = openingBalance;
	return [...summaries.values()]
		.sort((a, b) => a.periodKey.localeCompare(b.periodKey))
		.map((summary) => {
			rollingBalance += summary.netCashFlow;
			summary.endingBalance = rollingBalance;
			summary.categories = summary.categories.sort((a, b) => Math.abs(b.net) - Math.abs(a.net));
			return summary;
		})
		.reverse();
}

import type {
	Account,
	Category,
	CategoryAdjustment,
	LedgerEntry,
	PeriodCategoryTotal,
	PeriodGrain,
	PeriodSummary
} from './types';
import { entryAmountInBaseCurrency, isActive } from './utils';

export interface SpendingChangeValue {
	current: number;
	previous: number;
	delta: number;
	percent: number | null;
}

export interface SpendingChangeRow {
	categoryId: string | null;
	categoryName: string;
	categoryColor: string;
	monthToDate: number;
	day: SpendingChangeValue;
	month: SpendingChangeValue;
	year: SpendingChangeValue;
}

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

export function getCurrentBalance(accounts: Account[], entries: LedgerEntry[], baseCurrency = 'SGD'): number {
	return (
		getOpeningBalance(accounts) +
		entries
			.filter(isActive)
			.reduce(
				(sum, entry) =>
					sum + (entry.type === 'income' ? entryAmountInBaseCurrency(entry, baseCurrency) : -entryAmountInBaseCurrency(entry, baseCurrency)),
				0
			)
	);
}

export function buildPeriodSummaries(
	accounts: Account[],
	categories: Category[],
	entries: LedgerEntry[],
	adjustments: CategoryAdjustment[],
	grain: PeriodGrain,
	baseCurrency = 'SGD'
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
		const reportAmount = entryAmountInBaseCurrency(entry, baseCurrency);

		if (entry.type === 'expense') {
			summary.spent += reportAmount;
			categoryTotal.spent += reportAmount;
			categoryTotal.net -= reportAmount;
		} else {
			summary.income += reportAmount;
			categoryTotal.income += reportAmount;
			categoryTotal.net += reportAmount;
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

/**
 * Builds expense comparisons as of a local calendar date. Month and year
 * comparisons use equivalent elapsed ranges so partial periods stay useful.
 */
export function buildSpendingChanges(
	categories: Category[],
	entries: LedgerEntry[],
	asOfDate: string,
	baseCurrency = 'SGD'
): SpendingChangeRow[] {
	const asOf = parseLocalDate(asOfDate);
	const currentDay = dateRange(asOf, asOf);
	const previousDayDate = addLocalDays(asOf, -1);
	const previousDay = dateRange(previousDayDate, previousDayDate);

	const currentMonth = dateRange(new Date(asOf.getFullYear(), asOf.getMonth(), 1), asOf);
	const previousMonthStart = new Date(asOf.getFullYear(), asOf.getMonth() - 1, 1);
	const previousMonthEnd = new Date(
		previousMonthStart.getFullYear(),
		previousMonthStart.getMonth(),
		Math.min(asOf.getDate(), daysInMonth(previousMonthStart.getFullYear(), previousMonthStart.getMonth()))
	);
	const previousMonth = dateRange(previousMonthStart, previousMonthEnd);

	const currentYear = dateRange(new Date(asOf.getFullYear(), 0, 1), asOf);
	const previousYearEnd = new Date(
		asOf.getFullYear() - 1,
		asOf.getMonth(),
		Math.min(asOf.getDate(), daysInMonth(asOf.getFullYear() - 1, asOf.getMonth()))
	);
	const previousYear = dateRange(new Date(asOf.getFullYear() - 1, 0, 1), previousYearEnd);

	const expenseCategories = categories.filter((category) => category.type === 'expense' && isActive(category));
	const expenseCategoryIds = new Set(expenseCategories.map((category) => category.id));
	const amounts = new Map<string, number[]>();

	for (const category of expenseCategories) {
		amounts.set(category.id, [0, 0, 0, 0, 0, 0]);
	}

	for (const entry of entries) {
		if (!isActive(entry) || entry.type !== 'expense' || !expenseCategoryIds.has(entry.categoryId)) continue;
		const amount = entryAmountInBaseCurrency(entry, baseCurrency);
		const totals = amounts.get(entry.categoryId);
		if (!totals) continue;

		if (inRange(entry.occurredOn, currentDay)) totals[0] += amount;
		if (inRange(entry.occurredOn, previousDay)) totals[1] += amount;
		if (inRange(entry.occurredOn, currentMonth)) totals[2] += amount;
		if (inRange(entry.occurredOn, previousMonth)) totals[3] += amount;
		if (inRange(entry.occurredOn, currentYear)) totals[4] += amount;
		if (inRange(entry.occurredOn, previousYear)) totals[5] += amount;
	}

	const rows = expenseCategories.map((category) => {
		const totals = amounts.get(category.id) ?? [0, 0, 0, 0, 0, 0];
		return {
			categoryId: category.id,
			categoryName: category.name,
			categoryColor: category.color,
			monthToDate: totals[2],
			day: changeValue(totals[0], totals[1]),
			month: changeValue(totals[2], totals[3]),
			year: changeValue(totals[4], totals[5])
		};
	});

	const combinedTotals = [0, 0, 0, 0, 0, 0];
	for (const categoryTotals of amounts.values()) {
		categoryTotals.forEach((amount, index) => (combinedTotals[index] += amount));
	}
	return [
		{
			categoryId: null,
			categoryName: 'Total expense',
			categoryColor: '#64748b',
			monthToDate: combinedTotals[2],
			day: changeValue(combinedTotals[0], combinedTotals[1]),
			month: changeValue(combinedTotals[2], combinedTotals[3]),
			year: changeValue(combinedTotals[4], combinedTotals[5])
		},
		...rows
	];
}

type DateRange = { start: string; end: string };

function changeValue(current: number, previous: number): SpendingChangeValue {
	const delta = current - previous;
	return {
		current,
		previous,
		delta,
		percent: previous === 0 ? (current === 0 ? 0 : null) : (delta / previous) * 100
	};
}

function parseLocalDate(value: string): Date {
	const [year, month, day] = value.split('-').map(Number);
	return new Date(year, month - 1, day);
}

function addLocalDays(date: Date, amount: number): Date {
	return new Date(date.getFullYear(), date.getMonth(), date.getDate() + amount);
}

function daysInMonth(year: number, zeroBasedMonth: number): number {
	return new Date(year, zeroBasedMonth + 1, 0).getDate();
}

function dateRange(start: Date, end: Date): DateRange {
	return { start: localDateValue(start), end: localDateValue(end) };
}

function localDateValue(date: Date): string {
	const year = String(date.getFullYear()).padStart(4, '0');
	const month = String(date.getMonth() + 1).padStart(2, '0');
	const day = String(date.getDate()).padStart(2, '0');
	return `${year}-${month}-${day}`;
}

function inRange(date: string, range: DateRange): boolean {
	return date >= range.start && date <= range.end;
}

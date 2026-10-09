import type { Category, LedgerEntry } from './types';
import { currency, entryAmountInBaseCurrency, isActive, normalizeCurrencyCode } from './utils';

export interface MonthlySpending {
	key: string;
	label: string;
	spent: number;
	count: number;
	pendingCount: number;
	isCurrent: boolean;
}

export interface SpendingInsight {
	id: string;
	kind: 'coffee' | 'budget' | 'trend' | 'positive' | 'habit';
	title: string;
	description: string;
	context: string;
	saving?: number;
}

export interface CategorySpending {
	id: string;
	name: string;
	color: string;
	spent: number;
	count: number;
}

/** Callers supply only the current group's categories visible to this user. */
export function buildMonthlySpending(
	categories: Category[],
	entries: LedgerEntry[],
	asOfDate: string,
	months = 6,
	baseCurrency = 'SGD'
): MonthlySpending[] {
	const [year, month] = asOfDate.split('-').map(Number);
	const series: MonthlySpending[] = Array.from({ length: months }, (_, index) => {
		const date = new Date(year, month - months + index, 1);
		const key = monthKey(date.getFullYear(), date.getMonth() + 1);
		return {
			key,
			label: new Intl.DateTimeFormat(undefined, { month: 'short' }).format(date),
			spent: 0,
			count: 0,
			pendingCount: 0,
			isCurrent: key === asOfDate.slice(0, 7)
		};
	});
	const byMonth = new Map(series.map((item) => [item.key, item]));
	for (const entry of visibleExpenses(categories, entries, asOfDate)) {
		const point = byMonth.get(entry.occurredOn.slice(0, 7));
		if (!point) continue;
		if (awaitingConversion(entry, baseCurrency)) {
			point.pendingCount++;
			continue;
		}
		point.spent += entryAmountInBaseCurrency(entry, baseCurrency);
		point.count++;
	}
	return series;
}

export function buildMonthDetail(
	categories: Category[],
	entries: LedgerEntry[],
	selectedMonth: string,
	asOfDate: string,
	baseCurrency = 'SGD'
) {
	const [year, month] = selectedMonth.split('-').map(Number);
	const isCurrent = selectedMonth === asOfDate.slice(0, 7);
	const elapsedDays = isCurrent ? Number(asOfDate.slice(8, 10)) : monthDays(year, month);
	const previousDate = new Date(year, month - 2, 1);
	const previousKey = monthKey(previousDate.getFullYear(), previousDate.getMonth() + 1);
	const previousCutoff = Math.min(elapsedDays, monthDays(previousDate.getFullYear(), previousDate.getMonth() + 1));
	const visible = visibleExpenses(categories, entries, asOfDate);
	const currentEntries = visible.filter((entry) => entry.occurredOn.startsWith(selectedMonth));
	const previousEntries = visible.filter((entry) =>
		entry.occurredOn.startsWith(previousKey) && (!isCurrent || Number(entry.occurredOn.slice(8, 10)) <= previousCutoff)
	);
	const converted = (records: LedgerEntry[]) => records.filter((entry) => !awaitingConversion(entry, baseCurrency));
	const current = converted(currentEntries);
	const previous = converted(previousEntries);
	const sum = (records: LedgerEntry[]) => records.reduce((total, entry) => total + entryAmountInBaseCurrency(entry, baseCurrency), 0);
	const spent = sum(current);
	const previousSpent = sum(previous);
	const totals = new Map<string, CategorySpending>();
	const categoryMap = new Map(categories.map((category) => [category.id, category]));
	for (const entry of current) {
		const category = categoryMap.get(entry.categoryId)!;
		const total = totals.get(category.id) ?? { id: category.id, name: category.name, color: category.color, spent: 0, count: 0 };
		total.spent += entryAmountInBaseCurrency(entry, baseCurrency);
		total.count++;
		totals.set(category.id, total);
	}
	return {
		spent,
		previousSpent,
		delta: spent - previousSpent,
		percent: previousSpent > 0 ? ((spent - previousSpent) / previousSpent) * 100 : null,
		previousKey,
		previousCutoff,
		isCurrent,
		elapsedDays,
		daysInMonth: monthDays(year, month),
		current,
		previous,
		pendingCount: currentEntries.length - current.length,
		previousPendingCount: previousEntries.length - previous.length,
		categories: [...totals.values()].sort((a, b) => b.spent - a.spent)
	};
}

export function buildSpendingInsights(
	categories: Category[],
	entries: LedgerEntry[],
	selectedMonth: string,
	asOfDate: string,
	baseCurrency = 'SGD'
): SpendingInsight[] {
	const detail = buildMonthDetail(categories, entries, selectedMonth, asOfDate, baseCurrency);
	const { current, previous, elapsedDays, daysInMonth } = detail;
	// Wait for a useful sample; one purchase is not a spending habit.
	if (current.length < 3 || detail.pendingCount > 0) return [];
	const money = (amount: number) => currency(Math.round(amount), baseCurrency);
	const amount = (entry: LedgerEntry) => entryAmountInBaseCurrency(entry, baseCurrency);
	const period = detail.isCurrent ? 'so far this month' : 'this month';
	const insights: SpendingInsight[] = [];
	const categoryMap = new Map(categories.map((category) => [category.id, category]));
	const coffeePattern = /\b(coffee|cafe|café|kopi|starbucks|luckin)\b/i;
	const coffee = current.filter((entry) => coffeePattern.test(categoryMap.get(entry.categoryId)?.name ?? '') || coffeePattern.test(entry.merchant));
	const coffeeTotal = coffee.reduce((sum, entry) => sum + amount(entry), 0);
	if (coffee.length >= 4 && coffeeTotal > 0) {
		const saving = Math.round((coffeeTotal / coffee.length) * 4);
		insights.push({
			id: 'coffee', kind: 'coffee', title: 'A little less coffee, a little more saved',
			description: `${coffee.length} coffee shop purchases add up to ${money(coffeeTotal)} ${period}. Four fewer purchases in a month would save about ${money(saving)} at your average spend.`,
			context: 'An optional swap: make one coffee at home each week.', saving
		});
	}

	for (const total of detail.categories) {
		const category = categoryMap.get(total.id)!;
		if (!isActive(category) || category.monthlyTarget <= 0) continue;
		const target = category.monthlyTarget;
		if (total.spent > target) {
			insights.push({
				id: `budget-${total.id}`, kind: 'budget', title: `${total.name} spending is over target`,
				description: `${money(total.spent)} spent against a ${money(target)} monthly target — ${money(total.spent - target)} over.`,
				context: detail.isCurrent ? 'Review upcoming purchases or revisit this category’s target.' : 'Use this as a starting point for next month’s target.'
			});
		} else if (detail.isCurrent && elapsedDays >= 7 && total.count >= 3) {
			const projection = Math.round(total.spent / elapsedDays * daysInMonth);
			if (projection > target * 1.1) {
				insights.push({
					id: `pace-${total.id}`, kind: 'budget', title: `${total.name} spending is moving quickly`,
					description: `${money(total.spent)} of your ${money(target)} target is used. If daily spending continues at this pace, it would reach about ${money(projection)} by month end.`,
					context: `${money(target - total.spent)} remains in this category. One-off purchases can skew this estimate.`
				});
			}
		}
	}

	if (detail.previousPendingCount === 0) {
		const increases = detail.categories.map((total) => {
			const prior = previous.filter((entry) => entry.categoryId === total.id).reduce((sum, entry) => sum + amount(entry), 0);
			return { ...total, prior, increase: total.spent - prior };
		}).filter((total) => total.count >= 3 && total.prior > 0 && total.increase >= 1000 && total.increase / total.prior >= 0.25)
			.sort((a, b) => b.increase - a.increase);
		const rising = increases[0];
		if (rising && !insights.some((insight) => insight.id.endsWith(`-${rising.id}`))) {
			insights.push({
				id: `trend-${rising.id}`, kind: 'trend', title: `${rising.name} is your biggest jump`,
				description: `Spending is up ${Math.round(rising.increase / rising.prior * 100)}% (${money(rising.increase)}) compared with ${detail.isCurrent ? 'the same dates last month' : 'last month'}.`,
				context: 'Check the larger purchases to see whether this was a one-off or a new habit.'
			});
		}
	}

	if (insights.length < 3) {
		const merchants = new Map<string, { name: string; count: number; spent: number }>();
		for (const entry of current) {
			const name = entry.merchant.trim();
			if (!name || /^(untitled|unknown|other)$/i.test(name) || coffee.includes(entry)) continue;
			const key = name.toLowerCase().replace(/\s+/g, ' ');
			const merchant = merchants.get(key) ?? { name, count: 0, spent: 0 };
			merchant.count++;
			merchant.spent += amount(entry);
			merchants.set(key, merchant);
		}
		const repeat = [...merchants.values()].filter((merchant) => merchant.count >= 4 && merchant.spent > 0).sort((a, b) => b.spent - a.spent)[0];
		if (repeat) {
			const saving = Math.round(repeat.spent / repeat.count);
			insights.push({
				id: 'repeat-merchant', kind: 'habit', title: `A frequent stop: ${repeat.name}`,
				description: `${repeat.count} purchases total ${money(repeat.spent)} ${period}. If any are optional, one fewer purchase would save about ${money(saving)}.`,
				context: 'Based on your average purchase here.', saving
			});
		}
	}
	if (insights.length === 0 && previous.length >= 3 && detail.previousPendingCount === 0 && detail.previousSpent > 0 && detail.spent < detail.previousSpent * 0.9) {
		insights.push({
			id: 'lower-spending', kind: 'positive', title: 'Spending is running lower',
			description: `You’ve recorded ${money(-detail.delta)} less spending than ${detail.isCurrent ? 'the same dates last month' : 'last month'} (${Math.round(-detail.percent!)}% lower).`,
			context: 'A useful trend if both months’ transactions are up to date.'
		});
	}
	return insights.slice(0, 3);
}

function visibleExpenses(categories: Category[], entries: LedgerEntry[], asOfDate: string): LedgerEntry[] {
	// Archived categories retain their historical spending. Invisible personal
	// categories, deleted entries, income, and future transactions are excluded.
	const categoryMap = new Map(categories.map((category) => [category.id, category]));
	return entries.filter((entry) => {
		const category = categoryMap.get(entry.categoryId);
		return isActive(entry) && entry.type === 'expense' && category?.groupId === entry.groupId && entry.occurredOn <= asOfDate;
	});
}

function awaitingConversion(entry: LedgerEntry, baseCurrency: string): boolean {
	return entry.amount > 0 && normalizeCurrencyCode(entry.currency) !== normalizeCurrencyCode(baseCurrency) && entryAmountInBaseCurrency(entry, baseCurrency) === 0;
}

function monthKey(year: number, month: number): string {
	return `${year}-${String(month).padStart(2, '0')}`;
}

function monthDays(year: number, month: number): number {
	return new Date(year, month, 0).getDate();
}

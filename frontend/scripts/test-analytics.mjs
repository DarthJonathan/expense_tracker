import assert from 'node:assert/strict';
import { createServer } from 'vite';

const server = await createServer({ configFile: false, server: { middlewareMode: true }, appType: 'custom', logLevel: 'silent' });
try {
	const { buildMonthlySpending, buildMonthDetail, buildSpendingInsights } = await server.ssrLoadModule('/src/lib/analytics.ts');
	const category = (id, overrides = {}) => ({ id, groupId: 'group', name: id, type: 'expense', scope: 'household', color: '#2563eb', monthlyTarget: 0, createdAt: '', updatedAt: '', ...overrides });
	const entry = (id, categoryId, occurredOn, amount, overrides = {}) => ({
		id, categoryId, groupId: 'group', accountId: 'card', occurredOn, amount,
		type: 'expense', merchant: 'Example', note: '', currency: 'SGD', baseCurrency: 'SGD', baseAmount: amount,
		fxRate: 1, fxRateDate: occurredOn, createdAt: '', updatedAt: '', ...overrides
	});
	const categories = [category('food'), category('archived', { deletedAt: '2026-01-01' })];
	const records = [
		entry('jan', 'food', '2026-01-09', 1200), entry('oct', 'food', '2026-10-09', 1000),
		entry('converted', 'food', '2026-10-08', 500, { currency: 'USD', baseAmount: 650 }),
		entry('pending', 'food', '2026-10-08', 999999, { currency: 'USD', baseAmount: 0, fxRate: 0 }),
		entry('wrong-base', 'food', '2026-10-08', 999999, { currency: 'USD', baseCurrency: 'EUR' }),
		entry('archived-category', 'archived', '2026-10-08', 2000),
		entry('deleted', 'food', '2026-10-08', 999999, { deletedAt: '2026-10-09' }),
		entry('future', 'food', '2026-10-10', 999999),
		entry('income', 'food', '2026-10-08', 999999, { type: 'income' }),
		entry('invisible-category', 'private', '2026-10-08', 999999),
		entry('wrong-group', 'food', '2026-10-08', 999999, { groupId: 'other-group' })
	];
	const series = buildMonthlySpending(categories, records, '2026-10-09', 6);
	assert.deepEqual(series.map((month) => month.key), ['2026-05', '2026-06', '2026-07', '2026-08', '2026-09', '2026-10']);
	assert.ok(series.slice(0, 5).every((month) => month.spent === 0 && month.count === 0));
	assert.deepEqual({ spent: series[5].spent, count: series[5].count, pending: series[5].pendingCount, partial: series[5].isCurrent }, { spent: 3650, count: 3, pending: 2, partial: true });
	assert.equal(buildMonthlySpending(categories, records, '2026-01-09', 12)[0].key, '2025-02');
	assert.equal(buildMonthlySpending([], records, '2026-10-09').at(-1).spent, 0);
	assert.equal(buildMonthlySpending([categories[0]], records, '2026-10-09').at(-1).spent, 1650);
	assert.deepEqual(buildSpendingInsights(categories, records, '2026-10', '2026-10-09'), [], 'Pending FX must not produce misleading tips');

	const comparisonRecords = [
		entry('current', 'food', '2026-03-31', 3000),
		entry('previous-leap', 'food', '2026-02-28', 2000),
		entry('earlier', 'food', '2026-02-01', 500)
	];
	const clamped = buildMonthDetail(categories, comparisonRecords, '2026-03', '2026-03-31');
	assert.equal(clamped.previousSpent, 2500);
	assert.equal(clamped.elapsedDays, 31);
	const partial = buildMonthDetail(categories, [
		entry('current', 'food', '2026-10-09', 3000),
		entry('cutoff', 'food', '2026-09-09', 1000), entry('after-cutoff', 'food', '2026-09-10', 9000)
	], '2026-10', '2026-10-09');
	assert.equal(partial.previousSpent, 1000, 'Compare equal elapsed dates, not a partial month with a full month');
	assert.equal(partial.percent, 200);
	assert.equal(buildMonthDetail(categories, [entry('leap', 'food', '2024-02-29', 10)], '2024-02', '2024-03-15').daysInMonth, 29);
	assert.equal(buildMonthDetail(categories, [entry('new', 'food', '2026-01-01', 10)], '2026-01', '2026-01-09').previousKey, '2025-12');
	assert.equal(buildMonthDetail(categories, [], '2026-10', '2026-10-09').percent, null);

	const coffee = [1, 3, 5, 7].map((day, index) => entry(`coffee-${index}`, 'food', `2026-10-0${day}`, 600, { merchant: index % 2 ? 'STARBUCKS' : 'Local Coffee' }));
	const tips = buildSpendingInsights(categories, coffee, '2026-10', '2026-10-09');
	assert.equal(tips[0].kind, 'coffee');
	assert.equal(tips[0].saving, 2400);
	assert.match(tips[0].description, /4 coffee shop purchases/);
	assert.deepEqual(buildSpendingInsights(categories, coffee.slice(0, 2), '2026-10', '2026-10-09'), []);
	assert.deepEqual(buildSpendingInsights(categories, coffee, '2026-09', '2026-10-09'), [], 'Tips follow the selected month');
	assert.deepEqual(buildSpendingInsights([], coffee, '2026-10', '2026-10-09'), [], 'Filtered categories do not leak tips');

	const budgetCategories = [category('food', { monthlyTarget: 2000 })];
	const overBudget = buildSpendingInsights(budgetCategories, coffee, '2026-10', '2026-10-09');
	assert.ok(overBudget.some((insight) => insight.id === 'budget-food'));
	const pacing = buildSpendingInsights([category('food', { monthlyTarget: 6000 })], coffee, '2026-10', '2026-10-09');
	assert.ok(pacing.some((insight) => insight.id === 'pace-food'));
	assert.ok(!buildSpendingInsights([category('food', { monthlyTarget: 6000 })], coffee, '2026-10', '2026-10-06').some((insight) => insight.id === 'pace-food'), 'Do not project from the first few days');
	assert.ok(!buildSpendingInsights([category('food', { monthlyTarget: 6000, deletedAt: '2026-10-01' })], coffee, '2026-10', '2026-10-09').some((insight) => insight.kind === 'budget'), 'Archived targets are not current budgets');

	const rising = [1, 3, 5].flatMap((day) => [
		entry(`new-${day}`, 'food', `2026-10-0${day}`, 1500),
		entry(`old-${day}`, 'food', `2026-09-0${day}`, 500)
	]);
	assert.equal(buildSpendingInsights(categories, rising, '2026-10', '2026-10-09')[0].kind, 'trend');
	const declining = rising.map((record) => ({ ...record, amount: record.occurredOn.startsWith('2026-10') ? 300 : 1500 }));
	assert.equal(buildSpendingInsights(categories, declining, '2026-10', '2026-10-09')[0].kind, 'positive');
	const repeated = coffee.map((record) => ({ ...record, merchant: 'Example shop' }));
	assert.equal(buildSpendingInsights(categories, repeated, '2026-10', '2026-10-09')[0].kind, 'habit');
	console.log('analytics checks passed (calendar ranges, visibility, FX, filters, and spending suggestions)');
} finally {
	await server.close();
}

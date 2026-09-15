import assert from 'node:assert/strict';
import { createServer } from 'vite';

const server = await createServer({ configFile: false, server: { middlewareMode: true }, appType: 'custom', logLevel: 'silent' });

try {
	const { buildSpendingChanges } = await server.ssrLoadModule('/src/lib/reporting.ts');

	const category = (id, type = 'expense', overrides = {}) => ({
		id,
		groupId: 'group',
		name: id[0].toUpperCase() + id.slice(1),
		type,
		scope: 'household',
		color: '#2563eb',
		icon: '',
		monthlyTarget: 0,
		createdAt: '',
		updatedAt: '',
		...overrides
	});
	const entry = (id, categoryId, occurredOn, amount, overrides = {}) => ({
		id,
		groupId: 'group',
		accountId: 'cash',
		categoryId,
		type: 'expense',
		amount,
		currency: 'SGD',
		baseAmount: amount,
		baseCurrency: 'SGD',
		fxRate: 1,
		fxRateDate: occurredOn,
		occurredOn,
		merchant: id,
		note: '',
		createdAt: '',
		updatedAt: '',
		...overrides
	});

	const categories = [
		category('food'),
		category('transport'),
		category('health'),
		category('salary', 'income'),
		category('archived', 'expense', { deletedAt: '2026-01-01T00:00:00Z' })
	];
	const entries = [
		entry('food-today', 'food', '2026-09-15', 3000),
		entry('food-yesterday', 'food', '2026-09-14', 2000),
		entry('food-month-start', 'food', '2026-09-01', 7000),
		entry('food-previous-month-start', 'food', '2026-08-01', 4000),
		entry('food-previous-month-cutoff', 'food', '2026-08-15', 1000),
		entry('outside-previous-month-range', 'food', '2026-08-20', 9000),
		entry('food-current-year', 'food', '2026-01-10', 2000),
		entry('food-previous-year', 'food', '2025-01-10', 4000),
		entry('food-previous-year-cutoff', 'food', '2025-09-15', 3000),
		entry('outside-previous-year-range', 'food', '2025-09-16', 5000),
		entry('transport-yesterday', 'transport', '2026-09-14', 1000),
		entry('transport-month', 'transport', '2026-09-05', 2000),
		entry('income-is-not-spending', 'salary', '2026-09-15', 999999, { type: 'income' }),
		entry('inactive-category-is-excluded', 'archived', '2026-09-15', 999999),
		entry('deleted-entry-is-excluded', 'food', '2026-09-15', 999999, { deletedAt: '2026-09-15T12:00:00Z' })
	];

	const rows = buildSpendingChanges(categories, entries, '2026-09-15', 'SGD');
	assert.deepEqual(rows.map((row) => row.categoryName), ['Total expense', 'Food', 'Transport', 'Health']);

	const [total, food, transport, health] = rows;
	assert.equal(food.monthToDate, 12000);
	assert.deepEqual(food.day, { current: 3000, previous: 2000, delta: 1000, percent: 50 });
	assert.deepEqual(food.month, { current: 12000, previous: 5000, delta: 7000, percent: 140 });
	assert.deepEqual(food.year, { current: 28000, previous: 7000, delta: 21000, percent: 300 });

	assert.deepEqual(transport.day, { current: 0, previous: 1000, delta: -1000, percent: -100 });
	assert.deepEqual(transport.month, { current: 3000, previous: 0, delta: 3000, percent: null });
	assert.deepEqual(health.day, { current: 0, previous: 0, delta: 0, percent: 0 });
	assert.deepEqual(health.month, { current: 0, previous: 0, delta: 0, percent: 0 });
	assert.deepEqual(health.year, { current: 0, previous: 0, delta: 0, percent: 0 });

	assert.equal(total.monthToDate, 15000);
	assert.deepEqual(total.day, { current: 3000, previous: 3000, delta: 0, percent: 0 });
	assert.deepEqual(total.month, { current: 15000, previous: 5000, delta: 10000, percent: 200 });
	assert.deepEqual(total.year, { current: 31000, previous: 7000, delta: 24000, percent: 24000 / 7000 * 100 });

	const leapRows = buildSpendingChanges(
		[category('travel')],
		[
			entry('leap-day', 'travel', '2024-02-29', 1000),
			entry('prior-month-cutoff', 'travel', '2024-01-29', 500),
			entry('outside-prior-month-range', 'travel', '2024-01-30', 700),
			entry('prior-year-cutoff', 'travel', '2023-02-28', 250),
			entry('outside-prior-year-range', 'travel', '2023-03-01', 900)
		],
		'2024-02-29',
		'SGD'
	);
	const travel = leapRows[1];
	assert.deepEqual(travel.month, { current: 1000, previous: 500, delta: 500, percent: 100 });
	assert.deepEqual(travel.year, { current: 2200, previous: 250, delta: 1950, percent: 780 });

	console.log('reporting comparison checks passed');
} finally {
	await server.close();
}

import assert from 'node:assert/strict';
import { createServer } from 'vite';

const server = await createServer({ configFile: false, server: { middlewareMode: true }, appType: 'custom', logLevel: 'silent' });
try {
	const {
		combinedStatementRowsCompatibilityError,
		compatibleCombinedMatchEntries,
		countStatementReviews,
		filterStatementRows,
		matchedMasterUpdateCount,
		newMasterTransactionCount
	} = await server.ssrLoadModule('/src/lib/statement-review.ts');

	const row = (overrides = {}) => ({
		id: 'row-one', ingestionId: 'ingestion', groupId: 'group', sourceRowKey: 'page-1-row-1',
		occurredOn: '2026-08-01', merchant: 'Coffee Shop', amount: 1200, currency: 'SGD',
		foreignCurrency: '', statementKind: 'transaction', type: 'expense', cardholder: 'Primary Cardholder',
		statementReference: 'REF-100', accountId: 'card', categoryId: 'food', note: '', reviewStatus: 'unreviewed',
		warningCodes: [], createdAt: '', updatedAt: '', ...overrides
	});
	const entry = (overrides = {}) => ({
		id: 'expense-one', groupId: 'group', accountId: 'card', categoryId: 'food', type: 'expense',
		amount: 1200, currency: 'SGD', baseAmount: 1200, baseCurrency: 'SGD', fxRate: 1,
		fxRateDate: '2026-08-01', occurredOn: '2026-08-01', merchant: 'Coffee Shop', note: '',
		createdAt: '', updatedAt: '', ...overrides
	});

	const rows = [
		row(),
		row({ id: 'row-two', merchant: 'Train', cardholder: 'Secondary', statementReference: 'MRT-22', reviewStatus: 'new' }),
		row({ id: 'row-three', merchant: 'Refund', reviewStatus: 'ignored', type: 'income' })
	];
	assert.deepEqual(filterStatementRows(rows, 'secondary', 'all').map((item) => item.id), ['row-two']);
	assert.deepEqual(filterStatementRows(rows, 'mrt-22', 'new').map((item) => item.id), ['row-two']);
	assert.deepEqual(filterStatementRows(rows, '', 'ignored').map((item) => item.id), ['row-three']);
	assert.deepEqual(countStatementReviews(rows), { unreviewed: 1, new: 1, matched: 0, ignored: 1 });

	const matchRows = [
		row({ id: 'ordinary', reviewStatus: 'matched', matchExpenseId: 'master-one' }),
		row({ id: 'combined-one', reviewStatus: 'matched', matchExpenseId: 'master-two', combinedMatchId: 'group-one' }),
		row({ id: 'combined-two', reviewStatus: 'matched', matchExpenseId: 'master-two', combinedMatchId: 'group-one' }),
		row({ id: 'combined-three', reviewStatus: 'matched', matchExpenseId: 'master-three', combinedMatchId: 'group-two' }),
		row({ id: 'combined-four', reviewStatus: 'matched', matchExpenseId: 'master-three', combinedMatchId: 'group-two' }),
		row({ id: 'ignored-combined-label', reviewStatus: 'ignored', combinedMatchId: 'not-an-update' })
	];
	assert.equal(matchedMasterUpdateCount(matchRows), 3, 'each combined group must count as one master update');
	const newRows = [
		row({ id: 'new-one', reviewStatus: 'new', combinedMatchId: 'new-group' }),
		row({ id: 'new-two', reviewStatus: 'new', combinedMatchId: 'new-group' }),
		row({ id: 'ordinary-new', reviewStatus: 'new' }),
		...matchRows
	];
	assert.equal(newMasterTransactionCount(newRows), 2, 'combined sources create one transaction, plus the ordinary new row');
	assert.equal(countStatementReviews(newRows).new, 3, 'review progress still counts source rows');
	assert.equal(matchedMasterUpdateCount(newRows), 3, 'new groups do not increase matched update counts');
	assert.equal(newMasterTransactionCount([]), 0);

	assert.match(combinedStatementRowsCompatibilityError([row()]), /at least two/);
	assert.match(combinedStatementRowsCompatibilityError([row(), row({ id: 'two', accountId: null })]), /same currency, type, and account/);
	assert.match(combinedStatementRowsCompatibilityError([row(), row({ id: 'two', currency: 'USD' })]), /same currency, type, and account/);
	assert.match(combinedStatementRowsCompatibilityError([row(), row({ id: 'two', type: 'income' })]), /same currency, type, and account/);
	assert.match(combinedStatementRowsCompatibilityError([row(), row({ id: 'two', accountId: 'cash' })]), /same currency, type, and account/);
	assert.equal(combinedStatementRowsCompatibilityError([row(), row({ id: 'two' })]), null);

	const entries = [
		entry(),
		entry({ id: 'expense-two', merchant: 'Grocery', amount: 65352, occurredOn: '2026-08-15' }),
		entry({ id: 'wrong-account', accountId: 'cash' }),
		entry({ id: 'wrong-currency', currency: 'USD' }),
		entry({ id: 'wrong-type', type: 'income' })
	];
	const accounts = [{ id: 'card', groupId: 'group', name: 'DBS Vantage', type: 'card', openingBalance: 0, color: '', icon: '', createdAt: '', updatedAt: '' }];
	const categories = [{ id: 'food', groupId: 'group', name: 'Dining', type: 'expense', scope: 'household', color: '', icon: '', monthlyTarget: 0, createdAt: '', updatedAt: '' }];
	assert.deepEqual(compatibleCombinedMatchEntries([], entries, accounts, categories, '').map((item) => item.id), []);
	assert.deepEqual(compatibleCombinedMatchEntries([row()], entries, accounts, categories, '').map((item) => item.id), ['expense-one', 'expense-two']);
	assert.deepEqual(compatibleCombinedMatchEntries([row()], entries, accounts, categories, '653.52').map((item) => item.id), ['expense-two']);
	assert.deepEqual(compatibleCombinedMatchEntries([row()], entries, accounts, categories, 'dbs vantage').map((item) => item.id), ['expense-one', 'expense-two']);
	assert.deepEqual(compatibleCombinedMatchEntries([row()], entries, accounts, categories, 'dining').map((item) => item.id), ['expense-one', 'expense-two']);

	console.log('statement review checks passed');
} finally {
	await server.close();
}

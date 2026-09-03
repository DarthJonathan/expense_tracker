import assert from 'node:assert/strict';
import { createServer } from 'vite';

const server = await createServer({ configFile: false, server: { middlewareMode: true }, appType: 'custom', logLevel: 'silent' });
try {
	const { parseDbsStatementText } = await server.ssrLoadModule('/src/lib/statement-parser.ts');
	const statement = parseDbsStatementText(
		`===== PAGE 1 =====
STATEMENT DATE CREDIT LIMIT PAYMENT DUE DATE
14 Aug 2026 $1,000.00 08 Sep 2026
AMOUNT (S$)
PREVIOUS BALANCE 100.00
24 JUL BILL PAYMENT - INTERNET 50.00 CR
REF NO: TEST-REFERENCE
NEW TRANSACTIONS PRIMARY CARDHOLDER
11 JUL LOCAL SHOP 20.00
12 JUL MERCHANT REFUND 5.00 CR
SUB-TOTAL: 15.00
GRAND TOTAL FOR ALL CARD ACCOUNTS: 65.00`,
		'account-id',
		'redacted-fixture.pdf'
	);

	assert.equal(statement.statementDate, '2026-08-14');
	assert.equal(statement.paymentDueDate, '2026-09-08');
	assert.equal(statement.previousBalance, 10000);
	assert.equal(statement.declaredNewTransactionsTotal, 1500);
	assert.equal(statement.statementGrandTotal, 6500);
	assert.equal(statement.rows.length, 3);
	assert.equal(statement.rows[0].statementKind, 'payment');
	assert.equal(statement.rows[0].statementReference, 'TEST-REFERENCE');
	assert.equal(statement.rows[2].statementKind, 'refund');
	assert.deepEqual(statement.cardholders, [{ name: 'PRIMARY CARDHOLDER', declaredSubtotal: 1500 }]);

	const signedActivity = statement.rows.reduce(
		(total, row) => total + (row.type === 'income' ? -row.amount : row.amount),
		0
	);
	assert.equal(statement.previousBalance + signedActivity, statement.statementGrandTotal);
	console.log('statement parser checks passed');
} finally {
	await server.close();
}

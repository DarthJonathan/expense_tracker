export type AccountType = 'cash' | 'bank' | 'card' | 'wallet';
export type EntryType = 'expense' | 'income';
export type CategoryType = EntryType;
export type CategoryScope = 'household' | 'user';
export type PeriodGrain = 'day' | 'week' | 'month';

export interface SyncableRecord {
	id: string;
	groupId: string;
	createdAt: string;
	updatedAt: string;
	deletedAt?: string | null;
}

export interface Group extends Omit<SyncableRecord, 'groupId'> {
	name: string;
	inviteCode: string;
	createdBy?: string | null;
}

export interface Account extends SyncableRecord {
	name: string;
	type: AccountType;
	openingBalance: number;
	color: string;
	icon: string;
}

export interface Category extends SyncableRecord {
	name: string;
	type: CategoryType;
	scope: CategoryScope;
	ownerUserId?: string | null;
	color: string;
	icon: string;
	monthlyTarget: number;
}

export interface LedgerEntry extends SyncableRecord {
	accountId: string;
	categoryId: string;
	type: EntryType;
	amount: number;
	currency: string;
	baseAmount: number;
	baseCurrency: string;
	fxRate: number;
	fxRateDate: string;
	occurredOn: string;
	merchant: string;
	note: string;
	metadata?: Record<string, unknown>;
	createdBy?: string | null;
}

export interface CategoryAdjustment extends SyncableRecord {
	categoryId: string;
	amount: number;
	occurredOn: string;
	note: string;
}

export interface Merchant extends SyncableRecord {
	name: string;
	normalizedName: string;
	usageCount: number;
	lastUsedAt?: string | null;
}

export interface AppSettings {
	id: 'settings';
	activeGroupId: string;
	deviceUserId: string;
	baseCurrency: string;
	lastSyncedAt?: string | null;
}

export interface FinanceState {
	settings: AppSettings;
	groups: Group[];
	accounts: Account[];
	categories: Category[];
	entries: LedgerEntry[];
	adjustments: CategoryAdjustment[];
	merchants: Merchant[];
}

export interface PeriodCategoryTotal {
	periodKey: string;
	categoryId: string;
	categoryName: string;
	categoryColor: string;
	spent: number;
	income: number;
	adjustments: number;
	net: number;
}

export interface PeriodSummary {
	periodKey: string;
	income: number;
	spent: number;
	adjustments: number;
	netCashFlow: number;
	endingBalance: number;
	categories: PeriodCategoryTotal[];
}

export type StatementIngestionStatus = 'local_parsing' | 'parsed' | 'matching_review' | 'ready' | 'confirmed' | 'failed' | 'deleted';
export type StatementReviewStatus = 'unreviewed' | 'new' | 'matched' | 'ignored';
export type StatementKind = 'transaction' | 'payment' | 'fee' | 'refund' | 'other';

export interface StatementIngestion {
	id: string;
	groupId: string;
	accountId: string;
	clientRequestId?: string;
	sourceFingerprint?: string;
	status: Exclude<StatementIngestionStatus, 'local_parsing'>;
	sourceName: string;
	institution: string;
	statementCurrency: string;
	statementDate?: string | null;
	paymentDueDate?: string | null;
	periodStart?: string | null;
	periodEnd?: string | null;
	previousBalance?: number | null;
	declaredNewTransactionsTotal?: number | null;
	statementGrandTotal?: number | null;
	parsedRowCount: number;
	cardholderControls: Array<Record<string, unknown>>;
	validation: Record<string, unknown>;
	warnings: string[];
	createdBy?: string | null;
	confirmedAt?: string | null;
	createdAt: string;
	updatedAt: string;
	deletedAt?: string | null;
}

export interface StatementIngestionRow {
	id: string;
	ingestionId: string;
	groupId: string;
	sourceRowKey: string;
	occurredOn?: string | null;
	merchant: string;
	amount: number;
	currency: string;
	foreignAmount?: number | null;
	foreignCurrency: string;
	statementKind: StatementKind;
	type: EntryType;
	cardholder: string;
	statementReference: string;
	accountId?: string | null;
	categoryId?: string | null;
	note: string;
	reviewStatus: StatementReviewStatus;
	suggestedExpenseId?: string | null;
	matchConfidence?: number | null;
	matchExpenseId?: string | null;
	combinedMatchId?: string | null;
	combinedTransaction?: CombinedStatementTransaction | null;
	confirmedExpenseId?: string | null;
	warningCodes: string[];
	reviewedBy?: string | null;
	reviewedAt?: string | null;
	createdAt: string;
	updatedAt: string;
	deletedAt?: string | null;
}

export interface CombinedStatementTransaction {
	merchant: string;
	occurredOn: string;
	categoryId: string;
	note: string;
}

export interface StatementIngestionDetail {
	ingestion: StatementIngestion;
	rows: StatementIngestionRow[];
}

// Kept in IndexedDB only. The Blob is never included in an API request.
export interface LocalStatementJob {
	id: string;
	groupId: string;
	accountId: string;
	file: Blob;
	fileName: string;
	fileType: string;
	sourceFingerprint: string;
	status: Extract<StatementIngestionStatus, 'local_parsing' | 'failed'>;
	stage: 'embedded_text' | 'ocr' | 'structured_ready' | 'saving_structured' | 'failed';
	parsedPayload?: Record<string, unknown>;
	createdAt: string;
	updatedAt: string;
	error?: string;
}

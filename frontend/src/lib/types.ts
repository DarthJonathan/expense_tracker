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
	occurredOn: string;
	merchant: string;
	note: string;
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

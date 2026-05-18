import type { Account, Category, Group } from './types';
import { isoNow, makeId } from './utils';

export const DEFAULT_COLORS = ['#e7d24e', '#4b5745', '#df704f', '#c5caba', '#171a15', '#8f9984'];

export function createDefaultGroup(): Group {
	const now = isoNow();

	return {
		id: makeId(),
		name: 'Household',
		inviteCode: Math.random().toString(36).slice(2, 8).toUpperCase(),
		createdAt: now,
		updatedAt: now,
		deletedAt: null
	};
}

export function createDefaultAccounts(groupId: string): Account[] {
	const now = isoNow();

	return [
		{
			id: makeId(),
			groupId,
			name: 'Cash',
			type: 'cash',
			openingBalance: 0,
			color: '#e7d24e',
			icon: '💵',
			createdAt: now,
			updatedAt: now,
			deletedAt: null
		},
		{
			id: makeId(),
			groupId,
			name: 'Bank',
			type: 'bank',
			openingBalance: 0,
			color: '#4b5745',
			icon: '🏦',
			createdAt: now,
			updatedAt: now,
			deletedAt: null
		},
		{
			id: makeId(),
			groupId,
			name: 'Card',
			type: 'card',
			openingBalance: 0,
			color: '#df704f',
			icon: '💳',
			createdAt: now,
			updatedAt: now,
			deletedAt: null
		}
	];
}

export function createDefaultCategories(groupId: string): Category[] {
	const now = isoNow();
	const names = [
		['Groceries', 'expense', '#e7d24e', '🛒'],
		['Eating out', 'expense', '#df704f', '🍽️'],
		['Transport', 'expense', '#8f9984', '🚗'],
		['Home', 'expense', '#4b5745', '🏠'],
		['Health', 'expense', '#c5caba', '🩺'],
		['Income', 'income', '#171a15', '💼']
	] as const;

	return names.map(([name, type, color, icon]) => ({
		id: makeId(),
		groupId,
		name,
		type,
		color,
		icon,
		monthlyTarget: 0,
		createdAt: now,
		updatedAt: now,
		deletedAt: null
	}));
}

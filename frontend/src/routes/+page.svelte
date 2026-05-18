<script lang="ts">
	import {
		BarChart3,
		CalendarDays,
		ClipboardList,
		CreditCard,
		Home,
		Moon,
		Plus,
		RefreshCw,
		Search,
		Settings,
		Sun,
		Wifi,
		WifiOff
	} from 'lucide-svelte';
	import { DatePicker, Meter, Tabs } from 'bits-ui';
	import { parseDate, type DateValue } from '@internationalized/date';
	import AppSelect from '$lib/components/AppSelect.svelte';
	import { onMount } from 'svelte';
	import { buildPeriodSummaries, getCurrentBalance, periodKey, readablePeriod } from '$lib/reporting';
	import { finance } from '$lib/finance';
	import { clearSession, generateApiKey, getStoredSession, login, register, type APIKeyData, type AuthSession } from '$lib/auth';
	import { patchSettings } from '$lib/db';
	import { searchTransactionsRemote } from '$lib/transactions';
	import type { CategoryType, LedgerEntry, PeriodCategoryTotal, PeriodGrain } from '$lib/types';
	import { currency, formatSignedCurrency, isActive, todayInputValue } from '$lib/utils';

	type Screen = 'home' | 'review' | 'add' | 'transactions' | 'search' | 'transactionDetail' | 'settings';
	type DesktopScreen = 'dashboard' | 'transactions' | 'accounts' | 'review' | 'settings' | 'add';
	type StatItem = { name: string; color: string; amount: number };
	type BudgetComparisonItem = { id: string; name: string; spent: number; target: number; fillPercent: number };
	type FeedbackKind = 'success' | 'error';
	type SelectOption = { value: string; label: string; disabled?: boolean };

	const syncStatus = finance.syncStatus;
	const palette = ['#2563eb', '#10b981', '#8b5cf6', '#f59e0b', '#ef4444', '#0891b2', '#64748b', '#334155'];
	const legacyColorMap: Record<string, string> = {
		'#0f766e': '#10b981',
		'#2563eb': '#2563eb',
		'#c2410c': '#f59e0b',
		'#7c3aed': '#8b5cf6',
		'#be123c': '#ef4444',
		'#4d7c0f': '#0891b2'
	};

	let activeScreen: Screen = 'home';
	let desktopScreen: DesktopScreen = 'dashboard';
	let desktopAddAccountWizardOpen = false;
	let desktopAddCategoryWizardOpen = false;
	let transactionSearchQuery = '';
	let transactionSearchMonthsBack = '6';
	let transactionSearchOrigin: Screen = 'transactions';
	let remoteSearchResults: LedgerEntry[] | null = null;
	let remoteSearchError = '';
	let remoteSearchLoading = false;
	let remoteSearchRequestSeq = 0;
	let remoteSearchTimer: ReturnType<typeof setTimeout> | undefined;
	let searchRefreshKey = '';
	let selectedTransactionId = '';
	let selectedTransactionFallback: LedgerEntry | null = null;
	let selectedEntryType: 'expense' | 'income' = 'expense';
	let reviewEntryType: 'expense' | 'income' = 'expense';
	let grain: PeriodGrain = 'month';
	let selectedDate: DateValue = parseDate(todayInputValue());
	let selectedAccountId = '';
	let selectedCategoryId = '';
	let isOnline = true;
	let merchantQuery = '';
	let merchantSuggestions: string[] = [];
	let showMerchantSuggestions = false;
	let feedbackOpen = false;
	let feedbackKind: FeedbackKind = 'success';
	let feedbackTitle = '';
	let feedbackMessage = '';
	let feedbackTimer: ReturnType<typeof setTimeout> | undefined;
	let authSession: AuthSession | null = null;
	let authMode: 'login' | 'register' = 'login';
	let authLoading = false;
	let authError = '';
	let signupUnlocked = false;
	let authTapCount = 0;
	let unlockMessage = '';
	let authTapTimer: ReturnType<typeof setTimeout> | undefined;
	let generatedApiKey: APIKeyData | null = null;
	let generatingApiKey = false;
	let apiKeyFormError = '';
	let adjustmentCategoryId = '';
	let accountFormType = 'cash';
	let categoryFormType: CategoryType = 'expense';
	let themeMode: 'light' | 'dark' = 'light';
	const entryTypeOptions: SelectOption[] = [
		{ value: 'expense', label: 'Expense' },
		{ value: 'income', label: 'Income' }
	];
	const accountTypeOptions: SelectOption[] = [
		{ value: 'cash', label: 'Cash' },
		{ value: 'bank', label: 'Bank' },
		{ value: 'card', label: 'Card' },
		{ value: 'wallet', label: 'Wallet' }
	];
	const categoryTypeOptions: SelectOption[] = [
		{ value: 'expense', label: 'Expense' },
		{ value: 'income', label: 'Income' }
	];
	const grainOptions: SelectOption[] = [
		{ value: 'month', label: 'Monthly' },
		{ value: 'week', label: 'Weekly' },
		{ value: 'day', label: 'Daily' }
	];
	const transactionSearchHorizonOptions: SelectOption[] = [
		{ value: '1', label: '1 month' },
		{ value: '3', label: '3 months' },
		{ value: '6', label: '6 months' },
		{ value: '12', label: '12 months' },
		{ value: '24', label: '24 months' },
		{ value: 'all', label: 'All time' }
	];

	$: state = $finance;
	$: activeGroup = state?.groups.find((group) => group.id === state.settings.activeGroupId);
	$: accounts =
		state?.accounts
			.filter((account) => account.groupId === state.settings.activeGroupId && isActive(account))
			.map((account, index) => ({ ...account, color: displayColor(account.color, index) })) ?? [];
	$: categories =
		state?.categories
			.filter((category) => category.groupId === state.settings.activeGroupId && isActive(category))
			.map((category, index) => ({
				...category,
				type: normalizeCategoryType(category.type, category.name),
				color: displayColor(category.color, index + 1)
			})) ?? [];
	$: addCategoryOptions = getEntryCategoryOptions(categories, selectedEntryType);
	$: accountSelectOptions = accounts.map((account) => ({ value: account.id, label: account.name }));
	$: categorySelectOptions = addCategoryOptions.map((category) => ({ value: category.id, label: category.name }));
	$: settingsCategoryOptions = categories.map((category) => ({ value: category.id, label: category.name }));
	$: {
		if (settingsCategoryOptions.length === 0) {
			adjustmentCategoryId = '';
		} else if (!settingsCategoryOptions.some((option) => option.value === adjustmentCategoryId)) {
			adjustmentCategoryId = settingsCategoryOptions[0].value;
		}
	}
	$: {
		if (accounts.length === 0) {
			selectedAccountId = '';
		} else if (!accounts.some((account) => account.id === selectedAccountId)) {
			selectedAccountId = accounts[0].id;
		}
	}
	$: {
		if (addCategoryOptions.length === 0) {
			selectedCategoryId = '';
		} else if (!addCategoryOptions.some((category) => category.id === selectedCategoryId)) {
			selectedCategoryId = addCategoryOptions[0].id;
		}
	}
	$: entries = state?.entries.filter((entry) => entry.groupId === state.settings.activeGroupId && isActive(entry)) ?? [];
	$: adjustments =
		state?.adjustments.filter((adjustment) => adjustment.groupId === state.settings.activeGroupId && isActive(adjustment)) ??
		[];
	$: merchants = state?.merchants.filter((merchant) => merchant.groupId === state.settings.activeGroupId && isActive(merchant)) ?? [];
	$: summaries = buildPeriodSummaries(accounts, categories, entries, adjustments, grain);
	$: currentSummary = summaries[0];
	$: currentBalance = getCurrentBalance(accounts, entries);
	$: personalEntries = entries.filter((entry) => entry.createdBy === state?.settings.deviceUserId);
	$: personalIncome = personalEntries
		.filter((entry) => entry.type === 'income')
		.reduce((sum, entry) => sum + entry.amount, 0);
	$: personalSpent = personalEntries
		.filter((entry) => entry.type === 'expense')
		.reduce((sum, entry) => sum + entry.amount, 0);
	$: personalBalance = personalIncome - personalSpent;
	$: recentEntries = [...entries].sort((a, b) => b.occurredOn.localeCompare(a.occurredOn)).slice(0, 5);
	$: allTransactions = [...entries].sort((a, b) => `${b.occurredOn}${b.createdAt}`.localeCompare(`${a.occurredOn}${a.createdAt}`));
	$: selectedTransaction =
		allTransactions.find((entry) => entry.id === selectedTransactionId) ??
		(selectedTransactionFallback?.id === selectedTransactionId ? selectedTransactionFallback : null);
	$: transactionSearchQueryNormalized = normalizeMerchantText(transactionSearchQuery);
	$: filteredTransactions = filterTransactions(allTransactions, transactionSearchQueryNormalized, transactionSearchMonthsBack);
	$: searchPageResults = remoteSearchResults ?? filteredTransactions;
	$: topCategories = (currentSummary?.categories ?? []).filter((item) => Math.abs(item.net) > 0).slice(0, 4);
	$: homeTiles = topCategories.length
		? topCategories.map((item) => ({ name: item.categoryName, color: item.categoryColor, amount: Math.abs(item.net) }))
		: categories.slice(0, 2).map((item) => ({ name: item.name, color: item.color, amount: item.monthlyTarget }));
	$: statItems = buildStatItems(currentSummary?.categories ?? [], reviewEntryType);
	$: reviewTotal =
		reviewEntryType === 'income'
			? (currentSummary?.income ?? statItems.reduce((sum, item) => sum + item.amount, 0))
			: (currentSummary?.spent ?? statItems.reduce((sum, item) => sum + item.amount, 0));
	$: reviewTotalLabel = reviewEntryType === 'income' ? 'Total received' : 'Total spent';
	$: reviewTopTransactions = getReviewTopTransactions(entries, currentSummary?.periodKey, grain, reviewEntryType);
	$: reviewAnchorDate = getReviewAnchorDate(currentSummary?.periodKey, grain);
	$: reviewRailPrevLabel = getReviewRailLabel(reviewAnchorDate, grain, -1);
	$: reviewRailCenterLabel = getReviewRailLabel(reviewAnchorDate, grain, 0);
	$: reviewRailNextLabel = getReviewRailLabel(reviewAnchorDate, grain, 1);
	$: reviewGrainTabIndex = grain === 'month' ? 0 : grain === 'week' ? 1 : 2;
	$: entryTypeTabIndex = selectedEntryType === 'income' ? 1 : 0;
	$: expenseCategories = categories.filter((category) => normalizeCategoryType(category.type, category.name) === 'expense');
	$: budgetComparisonItems = buildBudgetComparisonItems(expenseCategories, currentSummary?.categories ?? []);
	$: currentCategoryTotals = new Map((currentSummary?.categories ?? []).map((item) => [item.categoryId, item]));
	$: homeCategoryProgress = categories.map((category) => {
		const totals = currentCategoryTotals.get(category.id);
		const categoryType = normalizeCategoryType(category.type, category.name);
		const amount = categoryType === 'income' ? totals?.income ?? 0 : totals?.spent ?? 0;
		const target = Math.max(0, category.monthlyTarget);
		const meterMax = target > 0 ? target : Math.max(amount, 1);
		const meterPercent = Math.min(100, Math.max(0, (amount / meterMax) * 100));

		return {
			...category,
			amount,
			target,
			meterMax,
			meterPercent
		};
	});
	$: desktopBalanceSeries = summaries.length
		? summaries.slice(0, 8).reverse()
		: [
				{ periodKey: '2026-01', endingBalance: 30 },
				{ periodKey: '2026-02', endingBalance: 48 },
				{ periodKey: '2026-03', endingBalance: 36 },
				{ periodKey: '2026-04', endingBalance: 68 },
				{ periodKey: '2026-05', endingBalance: 54 },
				{ periodKey: '2026-06', endingBalance: 76 },
				{ periodKey: '2026-07', endingBalance: 62 },
				{ periodKey: '2026-08', endingBalance: 72 }
			];
	$: latestBalanceSeriesIndex = Math.max(0, desktopBalanceSeries.length - 1);
	$: desktopHeading = getDesktopHeading(desktopScreen, activeGroup?.name);
	$: {
		if (typeof document !== 'undefined') {
			document.documentElement.dataset.theme = themeMode;
		}
	}

	onMount(() => {
		const syncWhenOnline = () => {
			isOnline = true;
			void finance.syncNow();
		};
		const markOffline = () => {
			isOnline = false;
		};

		void (async () => {
			const params = new URLSearchParams(window.location.search);
			if (params.get('signup') === '1') {
				signupUnlocked = true;
				authMode = 'register';
			}
			authSession = getStoredSession();
			const storedTheme = window.localStorage.getItem('spendit-theme');
			if (storedTheme === 'dark' || storedTheme === 'light') {
				themeMode = storedTheme;
			}
			await finance.init();
			if (authSession?.user?.id) {
				await patchSettings({ deviceUserId: authSession.user.id });
				await finance.init();
			}
			isOnline = navigator.onLine;
		})();

		window.addEventListener('online', syncWhenOnline);
		window.addEventListener('offline', markOffline);

		return () => {
			if (authTapTimer) clearTimeout(authTapTimer);
			if (remoteSearchTimer) clearTimeout(remoteSearchTimer);
			window.removeEventListener('online', syncWhenOnline);
			window.removeEventListener('offline', markOffline);
		};
	});

	async function submitAndSync(action: (formData: FormData) => Promise<void>, event: SubmitEvent) {
		const form = event.currentTarget as HTMLFormElement;
		const formData = new FormData(form);
		await action(formData);
		form.reset();
		selectedDate = parseDate(todayInputValue());
		merchantQuery = '';
		merchantSuggestions = [];
		showMerchantSuggestions = false;
		if (navigator.onLine) {
			void finance.syncNow();
		}
	}

	async function submitMovement(event: SubmitEvent, view: 'mobile' | 'desktop') {
		const form = event.currentTarget as HTMLFormElement;
		const formData = new FormData(form);

		try {
			await finance.addEntry(formData);
			form.reset();
			selectedDate = parseDate(todayInputValue());
			selectedEntryType = 'expense';
			merchantQuery = '';
			merchantSuggestions = [];
			showMerchantSuggestions = false;

			let synced = false;
			if (navigator.onLine) {
				synced = await finance.syncNow();
			}

			if (view === 'mobile') activeScreen = 'home';
			if (view === 'desktop') desktopScreen = 'dashboard';

			if (navigator.onLine && !synced) {
				showFeedback('error', 'Saved locally', 'Movement saved, but backend sync failed.');
				return;
			}

			if (!navigator.onLine) {
				showFeedback('success', 'Saved', 'Movement saved offline. Sync will happen when online.');
				return;
			}

			showFeedback('success', 'Saved', 'Movement has been saved successfully.');
		} catch (error) {
			showFeedback('error', 'Failed', error instanceof Error ? error.message : 'Unable to save movement.');
		}
	}

	async function submitDesktopAccountWizard(event: SubmitEvent) {
		try {
			await submitAndSync(finance.addAccount, event);
			desktopAddAccountWizardOpen = false;
			showFeedback('success', 'Account added', 'New account created successfully.');
		} catch (error) {
			showFeedback('error', 'Failed', error instanceof Error ? error.message : 'Unable to add account.');
		}
	}

	async function submitDesktopCategoryWizard(event: SubmitEvent) {
		try {
			await submitAndSync(finance.addCategory, event);
			desktopAddCategoryWizardOpen = false;
			showFeedback('success', 'Category added', 'New category created successfully.');
		} catch (error) {
			showFeedback('error', 'Failed', error instanceof Error ? error.message : 'Unable to add category.');
		}
	}

	function categoryName(id: string): string {
		return categories.find((category) => category.id === id)?.name ?? 'Uncategorized';
	}

	function accountName(id: string): string {
		return accounts.find((account) => account.id === id)?.name ?? 'Account';
	}

	function cutoffDateFromMonthsBack(monthsBack: string): string | null {
		if (monthsBack === 'all') return null;
		const months = Number(monthsBack);
		if (!Number.isFinite(months) || months <= 0) return null;
		const now = new Date();
		const cutoff = new Date(now.getFullYear(), now.getMonth() - months, now.getDate());
		return cutoff.toISOString().slice(0, 10);
	}

	function scoreTransactionMatch(entry: LedgerEntry, normalizedQuery: string): number {
		if (!normalizedQuery) return 0;
		const tokens = normalizedQuery.split(' ').filter(Boolean);
		if (!tokens.length) return 0;

		const normalizedAmount = normalizeMerchantText(String(entry.amount));
		const formattedAmount = normalizeMerchantText(currency(entry.amount));
		const fields = [
			normalizeMerchantText(entry.merchant),
			normalizeMerchantText(entry.note ?? ''),
			normalizeMerchantText(entry.occurredOn),
			normalizeMerchantText(entry.type),
			normalizeMerchantText(accountName(entry.accountId)),
			normalizeMerchantText(categoryName(entry.categoryId)),
			normalizedAmount,
			formattedAmount
		].filter(Boolean);

		let score = 0;
		for (const token of tokens) {
			let tokenBest = -1;
			for (const field of fields) {
				const fieldScore = merchantSearchScore(token, field);
				if (fieldScore > tokenBest) tokenBest = fieldScore;
			}
			if (tokenBest < 0) return -1;
			score += tokenBest;
		}

		const joined = fields.join(' ');
		if (joined.includes(normalizedQuery)) score += 18;
		return score;
	}

	function filterTransactions(entries: LedgerEntry[], normalizedQuery: string, monthsBack: string): LedgerEntry[] {
		const cutoffDate = cutoffDateFromMonthsBack(monthsBack);
		const windowedEntries = cutoffDate ? entries.filter((entry) => entry.occurredOn >= cutoffDate) : entries;
		if (!normalizedQuery) return windowedEntries;

		return windowedEntries
			.map((entry) => ({
				entry,
				score: scoreTransactionMatch(entry, normalizedQuery)
			}))
			.filter((item) => item.score >= 0)
			.sort((a, b) => {
				if (b.score !== a.score) return b.score - a.score;
				return `${b.entry.occurredOn}${b.entry.createdAt}`.localeCompare(`${a.entry.occurredOn}${a.entry.createdAt}`);
			})
			.map((item) => item.entry);
	}

	function openTransactionSearch(): void {
		transactionSearchOrigin = activeScreen;
		remoteSearchError = '';
		remoteSearchResults = null;
		activeScreen = 'search';
	}

	function closeTransactionSearch(): void {
		if (remoteSearchTimer) clearTimeout(remoteSearchTimer);
		remoteSearchError = '';
		remoteSearchLoading = false;
		remoteSearchResults = null;
		if (transactionSearchOrigin === 'search') {
			activeScreen = 'transactions';
			return;
		}
		activeScreen = transactionSearchOrigin;
	}

	function openTransactionDetail(entryId: string, fallback?: LedgerEntry): void {
		selectedTransactionId = entryId;
		selectedTransactionFallback = fallback ?? null;
		activeScreen = 'transactionDetail';
	}

	function scheduleRemoteTransactionSearch(): void {
		if (remoteSearchTimer) clearTimeout(remoteSearchTimer);
		remoteSearchTimer = setTimeout(() => {
			void runRemoteTransactionSearch();
		}, 220);
	}

	async function runRemoteTransactionSearch(): Promise<void> {
		if (!state || !authSession || !isOnline || activeScreen !== 'search') {
			remoteSearchResults = null;
			remoteSearchLoading = false;
			return;
		}

		const seq = ++remoteSearchRequestSeq;
		remoteSearchLoading = true;
		remoteSearchError = '';
		try {
			const monthsBack = transactionSearchMonthsBack === 'all' ? null : Number(transactionSearchMonthsBack);
			const results = await searchTransactionsRemote({
				groupId: state.settings.activeGroupId,
				query: transactionSearchQuery,
				monthsBack: Number.isFinite(monthsBack ?? NaN) ? monthsBack : null,
				limit: 100
			});
			if (seq !== remoteSearchRequestSeq) return;
			remoteSearchResults = results;
		} catch (error) {
			if (seq !== remoteSearchRequestSeq) return;
			remoteSearchResults = null;
			remoteSearchError = error instanceof Error ? error.message : 'Search failed';
		} finally {
			if (seq === remoteSearchRequestSeq) {
				remoteSearchLoading = false;
			}
		}
	}

	$: searchRefreshKey = `${activeScreen}|${transactionSearchQuery}|${transactionSearchMonthsBack}|${isOnline}|${authSession?.token ?? ''}|${state?.settings.activeGroupId ?? ''}`;
	$: if (searchRefreshKey && activeScreen === 'search') {
		scheduleRemoteTransactionSearch();
	}

	function entryIcon(label: string): string {
		return label.trim().slice(0, 1).toUpperCase() || '$';
	}

	function accountEmoji(account: { name: string; type: string; icon?: string }): string {
		const icon = account.icon?.trim();
		if (icon) return icon;
		const type = account.type.toLowerCase();
		if (type === 'cash') return '💵';
		if (type === 'card') return '💳';
		if (type === 'wallet') return '👛';
		return '🏦';
	}

	function categoryEmoji(category: { name: string; icon?: string }): string {
		const icon = category.icon?.trim();
		if (icon) return icon;
		const normalized = category.name.toLowerCase();
		if (normalized.includes('grocer') || normalized.includes('food') || normalized.includes('eat')) return '🍽️';
		if (normalized.includes('transport') || normalized.includes('fuel') || normalized.includes('car')) return '🚗';
		if (normalized.includes('home') || normalized.includes('rent')) return '🏠';
		if (normalized.includes('health') || normalized.includes('medic')) return '🩺';
		if (normalized.includes('income') || normalized.includes('salary')) return '💼';
		return '🏷️';
	}

	function formatDate(value: string): string {
		return new Intl.DateTimeFormat(undefined, { day: '2-digit', month: 'short' }).format(new Date(`${value}T00:00:00`));
	}

	function displayColor(color: string | undefined, index: number): string {
		const normalized = color?.trim().toLowerCase();
		if (normalized && legacyColorMap[normalized]) return legacyColorMap[normalized];
		if (normalized && palette.includes(normalized)) return normalized;
		return palette[index % palette.length];
	}

	function accountBalance(accountId: string): number {
		const account = accounts.find((item) => item.id === accountId);
		const movement = entries
			.filter((entry) => entry.accountId === accountId)
			.reduce((sum, entry) => sum + (entry.type === 'income' ? entry.amount : -entry.amount), 0);

		return (account?.openingBalance ?? 0) + movement;
	}

	function buildStatItems(items: PeriodCategoryTotal[], entryType: CategoryType): StatItem[] {
		const fromData = items
			.filter((item) => {
				const category = categories.find((record) => record.id === item.categoryId);
				return (
					normalizeCategoryType(category?.type, item.categoryName) === entryType &&
					(entryType === 'income' ? item.income : item.spent) > 0
				);
			})
			.slice(0, 8)
			.map((item) => ({
				name: item.categoryName,
				color: item.categoryColor,
				amount: entryType === 'income' ? item.income : item.spent
			}));

		if (fromData.length) return fromData;

		const realAmount = items.reduce((sum, item) => sum + (entryType === 'income' ? item.income : item.spent), 0);
		if (realAmount > 0) {
			return [{ name: entryType === 'income' ? 'Other income' : 'Other expense', color: '#6a6a61', amount: realAmount }];
		}

		if (entryType === 'income') {
			return [
				{ name: 'Salary', color: '#4b5745', amount: 120000 },
				{ name: 'Bonus', color: '#8f9984', amount: 35000 },
				{ name: 'Other', color: '#6a6a61', amount: 18000 }
			];
		}

		return [
			{ name: 'Home', color: '#4b5745', amount: 284000 },
			{ name: 'Food', color: '#df704f', amount: 107200 },
			{ name: 'Education', color: '#8f9984', amount: 45000 },
			{ name: 'Entertain.', color: '#e7d24e', amount: 59800 },
			{ name: 'Charity', color: '#171a15', amount: 52700 },
			{ name: 'Services', color: '#c5caba', amount: 82000 },
			{ name: 'Health', color: '#66735e', amount: 78500 },
			{ name: 'Clothes', color: '#b85f45', amount: 61000 },
			{ name: 'Other', color: '#6a6a61', amount: 1041700 }
		];
	}

	function buildBudgetComparisonItems(
		expenseOnlyCategories: typeof categories,
		periodTotals: PeriodCategoryTotal[]
	): BudgetComparisonItem[] {
		return expenseOnlyCategories.slice(0, 7).map((category) => {
			const total = periodTotals.find((item) => item.categoryId === category.id);
			const spent = total?.spent ?? 0;
			const target = category.monthlyTarget;
			const fillPercent = target > 0 ? (spent / target) * 100 : spent > 0 ? 100 : 0;
			return {
				id: category.id,
				name: category.name,
				spent,
				target,
				fillPercent: Math.max(spent > 0 ? 10 : 4, Math.min(100, fillPercent))
			};
		});
	}

	function normalizeCategoryType(value: string | undefined, nameFallback: string): CategoryType {
		const normalized = value?.toLowerCase().trim();
		if (normalized === 'income' || normalized === 'expense') {
			return normalized;
		}

		const guessed = nameFallback.toLowerCase();
		if (guessed.includes('income') || guessed.includes('salary') || guessed.includes('payroll')) {
			return 'income';
		}
		return 'expense';
	}

	function categoryTypeLabel(categoryType: CategoryType): string {
		return categoryType === 'income' ? 'Income' : 'Expense';
	}

	function getEntryCategoryOptions(
		allCategories: typeof categories,
		entryType: 'expense' | 'income'
	): typeof categories {
		const filtered = allCategories.filter((category) => normalizeCategoryType(category.type, category.name) === entryType);
		return filtered.length ? filtered : allCategories;
	}

	function ringSegmentStyle(items: StatItem[], index: number): string {
		const circumference = 565;
		const total = items.reduce((sum, item) => sum + item.amount, 0) || 1;
		const gap = 8;
		const previous = items.slice(0, index).reduce((sum, item) => sum + item.amount, 0);
		const rawLength = (items[index].amount / total) * circumference;
		const length = Math.max(4, rawLength - gap);
		const offset = circumference * 0.22 - (previous / total) * circumference - gap / 2;

		return `stroke:${items[index].color};stroke-dasharray:${length} ${circumference - length};stroke-dashoffset:${offset}`;
	}

	function shortPeriodLabel(key: string, periodGrain: PeriodGrain): string {
		if (periodGrain === 'month') {
			return new Intl.DateTimeFormat(undefined, { month: 'short' }).format(new Date(`${key}-01T00:00:00`));
		}

		if (periodGrain === 'week') {
			return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' }).format(new Date(`${key}T00:00:00`));
		}

		return new Intl.DateTimeFormat(undefined, { day: 'numeric' }).format(new Date(`${key}T00:00:00`));
	}

	function getReviewAnchorDate(periodKey: string | undefined, periodGrain: PeriodGrain): Date {
		if (periodKey) {
			return periodGrain === 'month' ? new Date(`${periodKey}-01T00:00:00`) : new Date(`${periodKey}T00:00:00`);
		}
		return new Date(`${todayInputValue()}T00:00:00`);
	}

	function shiftReviewDate(date: Date, periodGrain: PeriodGrain, offset: number): Date {
		const shifted = new Date(date);
		if (periodGrain === 'day') {
			shifted.setDate(shifted.getDate() + offset);
			return shifted;
		}
		if (periodGrain === 'week') {
			shifted.setDate(shifted.getDate() + offset * 7);
			return shifted;
		}
		shifted.setMonth(shifted.getMonth() + offset);
		return shifted;
	}

	function getReviewRailLabel(anchorDate: Date, periodGrain: PeriodGrain, offset: number): string {
		const date = shiftReviewDate(anchorDate, periodGrain, offset);
		if (periodGrain === 'day') {
			return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' }).format(date);
		}
		if (periodGrain === 'week') {
			const prefix = offset === 0 ? 'Week of ' : '';
			return `${prefix}${new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' }).format(date)}`;
		}
		return new Intl.DateTimeFormat(undefined, { month: 'short' }).format(date);
	}

	function getReviewTopTransactions(
		allEntries: LedgerEntry[],
		activePeriodKey: string | undefined,
		periodGrain: PeriodGrain,
		entryType: 'expense' | 'income'
	): LedgerEntry[] {
		if (!activePeriodKey) return [];
		return allEntries
			.filter((entry) => entry.type === entryType && periodKey(entry.occurredOn, periodGrain) === activePeriodKey)
			.sort((a, b) => {
				if (b.amount !== a.amount) return b.amount - a.amount;
				return `${b.occurredOn}${b.createdAt}`.localeCompare(`${a.occurredOn}${a.createdAt}`);
			})
			.slice(0, 5);
	}

	function getDesktopHeading(screen: DesktopScreen, groupName?: string): { title: string; subtitle: string } {
		if (screen === 'settings') {
			return {
				title: 'Settings',
				subtitle: `Group configuration and sync for ${groupName ?? 'your household'}.`
			};
		}
		if (screen === 'accounts') {
			return {
				title: 'Accounts',
				subtitle: `Balances, account setup, and categories for ${groupName ?? 'your household'}.`
			};
		}
		if (screen === 'transactions') {
			return {
				title: 'Transactions',
				subtitle: `All household ledger activity for ${groupName ?? 'your household'}.`
			};
		}
		if (screen === 'review') {
			return {
				title: 'Review',
				subtitle: `Category spending trends for ${groupName ?? 'your household'}.`
			};
		}
		if (screen === 'add') {
			return {
				title: 'Add transaction',
				subtitle: `Create an income or expense entry for ${groupName ?? 'your household'}.`
			};
		}
		return {
			title: 'Analytics',
			subtitle: `Detailed overview of ${groupName ?? 'your group'} financial activity.`
		};
	}

	function formatAmountInput(event: Event): void {
		const input = event.currentTarget as HTMLInputElement;
		const raw = input.value.trim();
		if (!raw) return;

		const unsigned = raw.startsWith('-') ? raw.slice(1) : raw;
		const negative = raw.startsWith('-');
		const hasDot = unsigned.includes('.');
		const [rawIntPart, rawFracPart = ''] = unsigned.split('.', 2);
		const intDigits = rawIntPart.replace(/\D/g, '');
		const fracDigits = rawFracPart.replace(/\D/g, '').slice(0, 2);

		if (!intDigits && !fracDigits) {
			input.value = negative ? '-' : '';
			return;
		}

		const intValue = intDigits ? Number(intDigits) : 0;
		const groupedInt = new Intl.NumberFormat(undefined, { maximumFractionDigits: 0 }).format(intValue);
		const sign = negative ? '-' : '';
		const decimalPart = hasDot ? `.${fracDigits}` : '';
		input.value = `${sign}${groupedInt}${decimalPart}`;
	}

	function normalizeMerchantText(value: string): string {
		return value
			.toLowerCase()
			.trim()
			.replace(/[^a-z0-9\s]/g, ' ')
			.replace(/\s+/g, ' ');
	}

	function merchantSearchScore(query: string, candidate: string): number {
		if (!query) return 0;
		if (candidate === query) return 100;
		if (candidate.startsWith(query)) return 80;
		if (candidate.includes(query)) return 60;

		let q = 0;
		for (let i = 0; i < candidate.length && q < query.length; i += 1) {
			if (candidate[i] === query[q]) q += 1;
		}

		return q === query.length ? 40 : -1;
	}

	function refreshMerchantSuggestions(rawQuery: string): void {
		const query = normalizeMerchantText(rawQuery);
		if (!query) {
			merchantSuggestions = [];
			showMerchantSuggestions = false;
			return;
		}
		const options = merchants
			.map((merchant) => ({ ...merchant, key: normalizeMerchantText(merchant.name) }))
			.filter((merchant) => merchant.name.trim().length > 0)
			.map((merchant) => ({
				name: merchant.name.trim(),
				usageCount: merchant.usageCount ?? 0,
				lastUsedAt: merchant.lastUsedAt ?? merchant.updatedAt,
				score: merchantSearchScore(query, merchant.key)
			}))
			.filter((merchant) => (query ? merchant.score >= 0 : true))
			.sort((a, b) => {
				if (b.score !== a.score) return b.score - a.score;
				if (b.usageCount !== a.usageCount) return b.usageCount - a.usageCount;
				return new Date(b.lastUsedAt).getTime() - new Date(a.lastUsedAt).getTime();
			});

		const uniqueNames = Array.from(new Set(options.map((merchant) => merchant.name)));
		merchantSuggestions = uniqueNames.slice(0, 8);
		showMerchantSuggestions = merchantSuggestions.length > 0;
	}

	function handleMerchantInput(event: Event): void {
		const input = event.currentTarget as HTMLInputElement;
		merchantQuery = input.value;
		refreshMerchantSuggestions(merchantQuery);
	}

	function handleMerchantFocus(): void {
		if (merchantQuery.trim().length > 0) {
			refreshMerchantSuggestions(merchantQuery);
		}
	}

	function handleMerchantBlur(): void {
		setTimeout(() => {
			showMerchantSuggestions = false;
		}, 120);
	}

	function applyMerchantSuggestion(value: string): void {
		merchantQuery = value;
		merchantSuggestions = [];
		showMerchantSuggestions = false;
	}

	function showFeedback(kind: FeedbackKind, title: string, message: string): void {
		if (feedbackTimer) clearTimeout(feedbackTimer);
		feedbackKind = kind;
		feedbackTitle = title;
		feedbackMessage = message;
		feedbackOpen = true;
		feedbackTimer = setTimeout(() => {
			feedbackOpen = false;
		}, 2200);
	}

	function closeFeedback(): void {
		if (feedbackTimer) clearTimeout(feedbackTimer);
		feedbackOpen = false;
	}

	function handleAuthHeadingTap(): void {
		if (signupUnlocked) return;
		authTapCount += 1;
		if (authTapTimer) clearTimeout(authTapTimer);
		authTapTimer = setTimeout(() => {
			authTapCount = 0;
		}, 1800);
		if (authTapCount >= 6) {
			signupUnlocked = true;
			authMode = 'register';
			authTapCount = 0;
			unlockMessage = 'Signup unlocked';
			authTapTimer = setTimeout(() => {
				unlockMessage = '';
			}, 2200);
		}
	}

	function toggleAuthMode(): void {
		if (!signupUnlocked) return;
		authMode = authMode === 'login' ? 'register' : 'login';
		authError = '';
	}

	async function submitAuth(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		const form = event.currentTarget as HTMLFormElement;
		const data = new FormData(form);
		const email = String(data.get('email') ?? '').trim();
		const password = String(data.get('password') ?? '');
		const displayName = String(data.get('displayName') ?? '').trim();

		authLoading = true;
		authError = '';
		try {
			authSession = authMode === 'register' ? await register(email, password, displayName) : await login(email, password);
			await patchSettings({ deviceUserId: authSession.user.id });
			await finance.init();
			if (navigator.onLine) {
				void finance.syncNow();
			}
			form.reset();
		} catch (error) {
			authError = error instanceof Error ? error.message : 'Authentication failed';
		} finally {
			authLoading = false;
		}
	}

	function logout(): void {
		clearSession();
		authSession = null;
		authError = '';
		generatedApiKey = null;
		apiKeyFormError = '';
	}

	function toggleTheme(): void {
		themeMode = themeMode === 'light' ? 'dark' : 'light';
		if (typeof window !== 'undefined') {
			window.localStorage.setItem('spendit-theme', themeMode);
		}
	}

	async function handleCreateApiKey(event: SubmitEvent): Promise<void> {
		event.preventDefault();
		const form = event.currentTarget as HTMLFormElement;
		const formData = new FormData(form);
		const name = String(formData.get('name') ?? '').trim();

		if (!name) {
			apiKeyFormError = 'Key name is required';
			return;
		}

		generatingApiKey = true;
		apiKeyFormError = '';
		try {
			generatedApiKey = await generateApiKey(name);
			showFeedback('success', 'API key created', 'Copy it now. This key is shown only once.');
			form.reset();
		} catch (error) {
			const message = error instanceof Error ? error.message : 'Unable to generate API key';
			apiKeyFormError = message;
			showFeedback('error', 'Generation failed', message);
		} finally {
			generatingApiKey = false;
		}
	}

	async function copyGeneratedApiKey(): Promise<void> {
		if (!generatedApiKey?.key) return;
		try {
			await navigator.clipboard.writeText(generatedApiKey.key);
			showFeedback('success', 'Copied', 'API key copied to clipboard');
		} catch {
			showFeedback('error', 'Copy failed', 'Please select and copy the key manually');
		}
	}
</script>

<svelte:head>
	<title>Shared Expense Tracker</title>
	<meta
		name="description"
		content="Offline-first shared group expense tracker with category, account and backend API sync support."
	/>
</svelte:head>

{#if !authSession}
	<div class="auth-overlay">
		<section class="auth-card">
			<button class="auth-heading" type="button" on:click={handleAuthHeadingTap}>
				{authMode === 'login' ? 'Sign in' : 'Create account'}
			</button>
			<p>{authMode === 'login' ? 'Sign in to sync with backend securely.' : 'Register to start secure household syncing.'}</p>
			<form on:submit|preventDefault={submitAuth}>
				<input name="email" type="email" placeholder="Email" required />
				<input name="password" type="password" placeholder="Password" minlength="8" required />
				{#if authMode === 'register'}
					<input name="displayName" placeholder="Display name" />
				{/if}
				{#if authError}
					<small class="auth-error">{authError}</small>
				{/if}
				<button type="submit" disabled={authLoading}>
					{authLoading ? 'Please wait...' : authMode === 'login' ? 'Login' : 'Register'}
				</button>
			</form>
			{#if signupUnlocked}
				<button class="ghost-auth" type="button" on:click={toggleAuthMode}>
					{authMode === 'login' ? 'Create account' : 'Have an account? Login'}
				</button>
			{/if}
			{#if unlockMessage}
				<small class="auth-unlock">{unlockMessage}</small>
			{/if}
		</section>
	</div>
{/if}

<main class="phone-app">
	{#if activeScreen === 'home'}
		<section class="screen home-screen">
			<header class="hero-header">
				<div>
					<p>Hello,</p>
					<h1>{activeGroup?.name ?? 'Household'}!</h1>
				</div>
				<div class="round-actions">
					<button title="Search transactions" type="button" on:click={openTransactionSearch}><Search size={22} /></button>
				</div>
			</header>

			<button class="balance-card" type="button" on:click={() => (activeScreen = 'add')}>
				<span>Household balance</span>
				<strong>{currency(currentBalance)}</strong>
			</button>

			<article class="balance-card personal-balance-card">
				<span>Personal balance</span>
				<strong>{currency(personalBalance)}</strong>
				<small>{personalEntries.length} of {entries.length} transactions</small>
			</article>

			<div class="home-stat-grid">
				<article>
					<span>Income</span>
					<strong>{currency(currentSummary?.income ?? 0)}</strong>
				</article>
				<article>
					<span>Spending</span>
					<strong>{currency(currentSummary?.spent ?? 0)}</strong>
				</article>
				<article>
					<span>Ending balance</span>
					<strong>{currency(currentSummary?.endingBalance ?? currentBalance)}</strong>
				</article>
				<article>
					<span>Accounts</span>
					<strong>{accounts.length}</strong>
				</article>
				<article>
					<span>Personal income</span>
					<strong>{currency(personalIncome)}</strong>
				</article>
				<article>
					<span>Personal expense</span>
					<strong>{currency(personalSpent)}</strong>
				</article>
			</div>

			<section class="section-heading">
				<h2>Top categories</h2>
				<button type="button" on:click={() => (activeScreen = 'review')}>Review</button>
			</section>
			<div class="payment-cards">
				{#each homeTiles as category, index}
					<article class:accent-card={index === 0} class="mini-card">
						<span style={`--swatch:${category.color}`}>
							{entryIcon(category.name)}
						</span>
						<h3>{category.name}</h3>
						<p>{currency(category.amount)}</p>
						<small>{grain} view</small>
					</article>
				{/each}
			</div>

			<section class="settings-list category-progress-list">
				<h2>Categories</h2>
				{#each homeCategoryProgress as category}
					<article>
						<span style={`--swatch:${category.color}`}></span>
						<div class="category-progress-content">
							<div class="category-progress-head">
								<div>
									<strong>{categoryEmoji(category)} {category.name}</strong>
									<small>{categoryTypeLabel(category.type)} · Target {currency(category.target)}</small>
								</div>
								<b>{currency(category.amount)}</b>
							</div>
							<Meter.Root
								class="category-progress-meter"
								value={category.amount}
								min={0}
								max={category.meterMax}
								aria-label={`${category.name} progress`}
								aria-valuetext={`${currency(category.amount)} of ${currency(category.target)} target`}
							>
								<div class="category-progress-fill" style={`--swatch:${category.color};transform:translateX(-${100 - category.meterPercent}%);`}></div>
							</Meter.Root>
						</div>
					</article>
				{/each}
			</section>

			<section class="section-heading">
				<h2>Recent Transactions</h2>
				<button type="button" on:click={() => (activeScreen = 'transactions')}>See all</button>
			</section>
			<div class="transaction-list">
				{#each recentEntries as entry}
					<article class="transaction-row">
						<div class="avatar">{entryIcon(entry.merchant)}</div>
						<div>
							<h3>{entry.merchant}</h3>
							<p>{formatDate(entry.occurredOn)} · {accountName(entry.accountId)}</p>
						</div>
						<strong class:negative={entry.type === 'expense'}>
							{entry.type === 'expense' ? '-' : '+'}{currency(entry.amount)}
						</strong>
					</article>
				{:else}
					<p class="empty-card">No activity yet.</p>
				{/each}
			</div>
		</section>
	{:else if activeScreen === 'review'}
		<section class="screen stats-screen">
			<header class="stats-header">
				<span class="header-spacer" aria-hidden="true"></span>
				<h1>Review (All accounts)</h1>
				<span class="header-spacer" aria-hidden="true"></span>
			</header>

			<div class="stats-controls">
				<AppSelect
					ariaLabel="Report type"
					bind:value={reviewEntryType}
					options={entryTypeOptions.map((option) => ({ ...option, label: `${option.label}s` }))}
					triggerClass="stats-select-trigger"
				/>
				<Tabs.Root bind:value={grain} class="stats-tabs-root">
					<Tabs.List class="stats-tabs" aria-label="Period range" style={`--active-index:${reviewGrainTabIndex}`}>
						<Tabs.Trigger value="month">Month</Tabs.Trigger>
						<Tabs.Trigger value="week">Week</Tabs.Trigger>
						<Tabs.Trigger value="day">Day</Tabs.Trigger>
					</Tabs.List>
				</Tabs.Root>
			</div>

			<div class="month-rail">
				<span>{reviewRailPrevLabel}</span>
				<strong>{reviewRailCenterLabel}</strong>
				<span>{reviewRailNextLabel}</span>
			</div>

			<section class="stat-ring-card">
				<div class="ring-wrap">
					<svg class="ring-svg" viewBox="0 0 220 220" aria-label="Category expense ring">
						<circle class="ring-track" cx="110" cy="110" r="90" pathLength="565" />
						{#each statItems as item, index}
							<circle class="ring-segment" cx="110" cy="110" r="90" pathLength="565" style={ringSegmentStyle(statItems, index)} />
						{/each}
					</svg>
					<div>
						<strong>{currency(reviewTotal)}</strong>
						<span>{currentSummary ? reviewTotalLabel : 'Sample stats'}</span>
					</div>
				</div>
			</section>

			<div class="stat-legend">
				{#each statItems as item}
					<div>
						<span style={`--swatch:${item.color}`}></span>
						<p>{item.name}</p>
						<strong>{currency(item.amount)}</strong>
					</div>
				{/each}
			</div>

			<section class="review-top-transactions">
				<div class="section-heading">
					<h2>Top Transactions</h2>
					<span>{grain === 'day' ? 'Today' : grain === 'week' ? 'This week' : 'This month'}</span>
				</div>
				<div class="transaction-list">
					{#each reviewTopTransactions as entry}
						<article class="transaction-row">
							<div class="avatar">{entryIcon(entry.merchant)}</div>
							<div>
								<h3>{entry.merchant}</h3>
								<p>{formatDate(entry.occurredOn)} · {accountName(entry.accountId)}</p>
							</div>
							<strong class:negative={entry.type === 'expense'}>
								{entry.type === 'expense' ? '-' : '+'}{currency(entry.amount)}
							</strong>
						</article>
					{:else}
						<p class="empty-card">No {reviewEntryType} transactions in this period.</p>
					{/each}
				</div>
			</section>

			<button class="history-pill" type="button">History</button>
		</section>
	{:else if activeScreen === 'add'}
			<section class="screen">
				<header class="screen-header">
					<span class="header-spacer"></span>
					<h1>Add Transaction</h1>
					<span class="header-spacer"></span>
				</header>

				<form class="form-card" on:submit|preventDefault={(event) => submitMovement(event, 'mobile')}>
					<Tabs.Root bind:value={selectedEntryType} class="entry-type-tabs-root">
						<Tabs.List class="entry-type-tabs" aria-label="Entry type" style={`--active-index:${entryTypeTabIndex}`}>
							<Tabs.Trigger value="expense" class="entry-type-tab">Expense</Tabs.Trigger>
							<Tabs.Trigger value="income" class="entry-type-tab">Income</Tabs.Trigger>
						</Tabs.List>
					</Tabs.Root>
					<input name="type" type="hidden" value={selectedEntryType} />
				<label>
					Amount
					<input name="amount" type="text" inputmode="decimal" placeholder="0.00" on:input={formatAmountInput} required />
				</label>
				<label class="merchant-field">
					Merchant or source
					<input
						bind:value={merchantQuery}
						autocomplete="off"
						name="merchant"
						on:blur={handleMerchantBlur}
						on:focus={handleMerchantFocus}
						on:input={handleMerchantInput}
						placeholder="Supermarket, salary, transfer..."
						required
					/>
					{#if showMerchantSuggestions}
						<div class="merchant-suggestions" role="listbox" aria-label="Merchant suggestions">
							{#each merchantSuggestions as merchantName}
								<button type="button" on:mousedown|preventDefault={() => applyMerchantSuggestion(merchantName)}>
									{merchantName}
								</button>
							{/each}
						</div>
					{/if}
				</label>
				<div class="field-grid">
					<label>
						Account
						<AppSelect
							ariaLabel="Account"
							bind:value={selectedAccountId}
							disabled={accountSelectOptions.length === 0}
							name="accountId"
							options={accountSelectOptions}
							placeholder="No accounts configured"
							required
						/>
					</label>
					<label>
						Category
						<AppSelect
							ariaLabel="Category"
							bind:value={selectedCategoryId}
							disabled={categorySelectOptions.length === 0}
							name="categoryId"
							options={categorySelectOptions}
							placeholder="No categories configured"
							required
						/>
					</label>
				</div>
				<div class="field-grid">
					<label>
						Date
						<DatePicker.Root bind:value={selectedDate} weekdayFormat="short" fixedWeeks={true}>
							<div class="date-picker-field">
								<DatePicker.Input name="occurredOn" class="date-picker-input">
									{#snippet children({ segments })}
										{#each segments as segment, index (`${segment.part}-${index}`)}
											{#if segment.part === 'literal'}
												<span class="date-picker-literal">{segment.value}</span>
											{:else}
												<DatePicker.Segment part={segment.part} class="date-picker-segment">
													{segment.value}
												</DatePicker.Segment>
											{/if}
										{/each}
									{/snippet}
								</DatePicker.Input>
								<DatePicker.Trigger class="date-picker-trigger" aria-label="Open calendar">
									<CalendarDays size={18} />
								</DatePicker.Trigger>
							</div>
							<DatePicker.Portal>
								<DatePicker.Content class="date-picker-content" sideOffset={8} align="end">
									<DatePicker.Calendar class="date-picker-calendar">
										{#snippet children({ months, weekdays })}
											<DatePicker.Header class="date-picker-calendar-header">
												<DatePicker.PrevButton class="date-picker-nav-button" aria-label="Previous month">‹</DatePicker.PrevButton>
												<DatePicker.Heading class="date-picker-heading" />
												<DatePicker.NextButton class="date-picker-nav-button" aria-label="Next month">›</DatePicker.NextButton>
											</DatePicker.Header>
											{#each months as month}
												<DatePicker.Grid class="date-picker-grid">
													<DatePicker.GridHead>
														<DatePicker.GridRow class="date-picker-grid-row">
															{#each weekdays as day}
																<DatePicker.HeadCell class="date-picker-head-cell">{day}</DatePicker.HeadCell>
															{/each}
														</DatePicker.GridRow>
													</DatePicker.GridHead>
													<DatePicker.GridBody>
														{#each month.weeks as weekDates}
															<DatePicker.GridRow class="date-picker-grid-row">
																{#each weekDates as date}
																	<DatePicker.Cell {date} month={month.value}>
																		<DatePicker.Day class="date-picker-day" />
																	</DatePicker.Cell>
																{/each}
															</DatePicker.GridRow>
														{/each}
													</DatePicker.GridBody>
												</DatePicker.Grid>
											{/each}
										{/snippet}
									</DatePicker.Calendar>
								</DatePicker.Content>
							</DatePicker.Portal>
						</DatePicker.Root>
					</label>
					<label>
						Note
						<input name="note" placeholder="Optional" />
					</label>
				</div>
				<button class="primary-button" type="submit" disabled={!accounts.length || !addCategoryOptions.length}>
					<span class="sr-only">Save movement</span>
					<Plus size={20} aria-hidden="true" />
				</button>
			</form>
		</section>
	{:else if activeScreen === 'transactions'}
			<section class="screen">
				<header class="screen-header">
					<span class="header-spacer"></span>
					<h1>Transactions</h1>
					<button class="plain-icon-button" title="Search transactions" type="button" on:click={openTransactionSearch}>
						<Search size={18} />
					</button>
			</header>

			<section class="transaction-table-card">
				<div class="transaction-table-head">
					<span>Date</span>
					<span>Merchant</span>
					<span>Category</span>
					<span>Amount</span>
				</div>
				<div class="transaction-table">
					{#each allTransactions as entry}
						<button
							type="button"
							aria-label={`Open transaction ${entry.merchant} ${currency(entry.amount)}`}
							on:click={() => openTransactionDetail(entry.id)}
						>
							<time>{formatDate(entry.occurredOn)}</time>
							<div>
								<strong>{entry.merchant}</strong>
								<small>{accountName(entry.accountId)}</small>
							</div>
							<span>{categoryName(entry.categoryId)}</span>
							<b class:negative={entry.type === 'expense'}>
								{entry.type === 'expense' ? '-' : '+'}{currency(entry.amount)}
							</b>
						</button>
					{:else}
						<p class="empty-card">No transactions yet.</p>
					{/each}
				</div>
			</section>
		</section>
	{:else if activeScreen === 'search'}
			<section class="screen">
				<header class="screen-header">
					<span class="header-spacer"></span>
					<h1>Search Transactions</h1>
					<button class="plain-icon-button" type="button" title="Close search" aria-label="Close search" on:click={closeTransactionSearch}>×</button>
				</header>
			<section class="transaction-table-card">
				<div class="search-screen-controls">
					<input bind:value={transactionSearchQuery} placeholder="Merchant, category, account, amount..." aria-label="Search transactions" />
					<AppSelect ariaLabel="Search horizon" bind:value={transactionSearchMonthsBack} options={transactionSearchHorizonOptions} />
				</div>
				<div class="search-screen-results">
					{#if remoteSearchLoading}
						<p class="empty-card">Searching...</p>
					{/if}
					{#if remoteSearchError}
						<p class="auth-error">{remoteSearchError}</p>
					{/if}
					{#each searchPageResults as entry}
						<button
							type="button"
							aria-label={`Open transaction ${entry.merchant} ${currency(entry.amount)}`}
							on:click={() => openTransactionDetail(entry.id, entry)}
						>
							<div>
								<strong>{entry.merchant}</strong>
								<small>{formatDate(entry.occurredOn)} · {categoryName(entry.categoryId)} · {accountName(entry.accountId)}</small>
							</div>
							<b class:negative={entry.type === 'expense'}>
								{entry.type === 'expense' ? '-' : '+'}{currency(entry.amount)}
							</b>
						</button>
					{:else}
						<p class="empty-card">
							{transactionSearchQueryNormalized
								? `No match in last ${transactionSearchMonthsBack === 'all' ? 'all time' : `${transactionSearchMonthsBack} month(s)`}.`
								: 'No transactions in selected period.'}
						</p>
					{/each}
				</div>
			</section>
		</section>
	{:else if activeScreen === 'transactionDetail'}
			<section class="screen">
				<header class="screen-header">
					<span class="header-spacer"></span>
					<h1>Transaction Detail</h1>
					<button class="plain-icon-button" title="Close" type="button" on:click={() => (activeScreen = 'transactions')}>×</button>
				</header>
			{#if selectedTransaction}
				<section class="transaction-detail-card">
					<div class="transaction-detail-row">
						<span>Merchant</span>
						<strong>{selectedTransaction.merchant}</strong>
					</div>
					<div class="transaction-detail-row">
						<span>Type</span>
						<strong>{selectedTransaction.type === 'expense' ? 'Expense' : 'Income'}</strong>
					</div>
					<div class="transaction-detail-row">
						<span>Amount</span>
						<strong class:negative={selectedTransaction.type === 'expense'}>
							{selectedTransaction.type === 'expense' ? '-' : '+'}{currency(selectedTransaction.amount)}
						</strong>
					</div>
					<div class="transaction-detail-row">
						<span>Currency</span>
						<strong>{selectedTransaction.currency}</strong>
					</div>
					<div class="transaction-detail-row">
						<span>Date</span>
						<strong>{formatDate(selectedTransaction.occurredOn)}</strong>
					</div>
					<div class="transaction-detail-row">
						<span>Category</span>
						<strong>{categoryName(selectedTransaction.categoryId)}</strong>
					</div>
					<div class="transaction-detail-row">
						<span>Account</span>
						<strong>{accountName(selectedTransaction.accountId)}</strong>
					</div>
					<div class="transaction-detail-row">
						<span>Note</span>
						<strong>{selectedTransaction.note || '-'}</strong>
					</div>
				</section>
			{:else}
				<p class="empty-card">Transaction not found.</p>
			{/if}
		</section>
	{:else}
			<section class="screen settings-screen">
				<header class="screen-header">
					<span class="header-spacer"></span>
					<h1>Settings</h1>
					<span class:offline={!isOnline} class="connection">
					{#if isOnline}<Wifi size={16} />{:else}<WifiOff size={16} />{/if}
				</span>
			</header>

			<div class="quick-menu">
				<article>
					<CreditCard size={24} />
					<h3>{accounts.length} accounts</h3>
					<p>{accounts.map((account) => account.name).join(', ')}</p>
				</article>
				<article>
					<BarChart3 size={24} />
					<h3>{categories.length} categories</h3>
					<p>{categories.slice(0, 3).map((category) => category.name).join(', ')}</p>
				</article>
			</div>

			<section class="settings-list">
				<h2>Account information</h2>
				{#each accounts as account}
					<article>
						<span style={`--swatch:${account.color}`}></span>
						<div>
							<strong>{accountEmoji(account)} {account.name}</strong>
							<small>{account.type}</small>
						</div>
						<b>{currency(accountBalance(account.id))}</b>
					</article>
				{/each}
			</section>

			<form class="form-card" on:submit|preventDefault={(event) => submitAndSync(finance.addAdjustment, event)}>
				<h2>Manual category add/minus</h2>
				<AppSelect
					ariaLabel="Adjustment category"
					bind:value={adjustmentCategoryId}
					disabled={settingsCategoryOptions.length === 0}
					name="categoryId"
					options={settingsCategoryOptions}
					placeholder="No categories configured"
					required
				/>
				<div class="field-grid">
					<input name="amount" type="text" inputmode="decimal" placeholder="+/- amount" on:input={formatAmountInput} required />
					<input name="occurredOn" type="date" value={todayInputValue()} />
				</div>
				<input name="note" placeholder="Adjustment note" />
				<button type="submit">Adjust category</button>
			</form>

			<form class="form-card" on:submit|preventDefault={(event) => submitAndSync(finance.addAccount, event)}>
				<h2>Add account</h2>
				<input name="name" placeholder="Account name" required />
				<div class="field-grid">
					<AppSelect ariaLabel="Account type" bind:value={accountFormType} name="type" options={accountTypeOptions} />
					<input name="openingBalance" type="text" inputmode="decimal" placeholder="Opening" on:input={formatAmountInput} />
				</div>
				<input name="icon" placeholder="Emoji icon (e.g. 🏦)" />
				<input name="color" type="color" value="#2563eb" title="Account color" />
				<button type="submit">Add account</button>
			</form>

			<form class="form-card" on:submit|preventDefault={(event) => submitAndSync(finance.addCategory, event)}>
				<h2>Add category</h2>
				<input name="name" placeholder="Category name" required />
				<div class="field-grid">
					<AppSelect ariaLabel="Category type" bind:value={categoryFormType} name="type" options={categoryTypeOptions} />
					<input name="monthlyTarget" type="text" inputmode="decimal" placeholder="Monthly target" on:input={formatAmountInput} />
				</div>
				<input name="icon" placeholder="Emoji icon (e.g. 🛒)" />
				<input name="color" type="color" value="#10b981" title="Category color" />
				<button type="submit">Add category</button>
			</form>

			<form class="form-card" on:submit|preventDefault={(event) => submitAndSync(finance.updateGroupName, event)}>
				<h2>Group</h2>
				<input name="name" value={activeGroup?.name ?? ''} placeholder="Group name" />
				<p class="muted">Invite code {activeGroup?.inviteCode ?? 'LOCAL'}</p>
				<button type="submit">Rename group</button>
			</form>

			<section class="form-card">
				<h2>Appearance</h2>
				<p class="muted">Current mode: {themeMode === 'dark' ? 'Dark' : 'Light'}</p>
				<button class="ghost theme-toggle-button" type="button" on:click={toggleTheme}>
					{#if themeMode === 'dark'}
						<Sun size={16} />
						Switch to light mode
					{:else}
						<Moon size={16} />
						Switch to dark mode
					{/if}
				</button>
			</section>

			<section class="form-card">
				<h2>Server sync</h2>
				<p class="muted">{$syncStatus.message}</p>
				<p class="muted">Signed in as {authSession?.user.email}</p>
				<div class="button-row">
					<button type="button" on:click={() => finance.syncNow()}>
						<RefreshCw size={16} />
						Sync now
					</button>
					<button class="ghost" type="button" on:click={logout}>Logout</button>
				</div>
			</section>

			<form class="form-card" on:submit|preventDefault={handleCreateApiKey}>
				<h2>API keys</h2>
				<p class="muted">Generate API keys for scripts or backend callers.</p>
				<input name="name" placeholder="Key name (e.g. Household sync script)" required />
				<button type="submit" disabled={generatingApiKey}>{generatingApiKey ? 'Generating...' : 'Generate API key'}</button>
				{#if apiKeyFormError}
					<p class="auth-error">{apiKeyFormError}</p>
				{/if}
				{#if generatedApiKey}
					<div class="generated-api-key">
						<p>Copy now (shown once)</p>
						<input value={generatedApiKey.key} readonly />
						<button type="button" on:click={copyGeneratedApiKey}>Copy key</button>
					</div>
				{/if}
			</form>
		</section>
	{/if}

	<nav class="bottom-nav" aria-label="Primary">
		<button class:active={activeScreen === 'home'} title="Home" type="button" on:click={() => (activeScreen = 'home')}>
			<Home size={23} />
			<span>Home</span>
		</button>
		<button
			class:active={activeScreen === 'review'}
			title="Review"
			type="button"
			on:click={() => (activeScreen = 'review')}
		>
			<BarChart3 size={23} />
			<span>Review</span>
		</button>
		<button
			class="fab"
			class:active={activeScreen === 'add'}
			aria-label="Add transaction"
			type="button"
			on:click={() => (activeScreen = 'add')}
		>
			<Plus size={30} />
			<span>Add</span>
		</button>
		<button
			class:active={activeScreen === 'transactions' || activeScreen === 'search' || activeScreen === 'transactionDetail'}
			title="Transactions"
			type="button"
			on:click={() => (activeScreen = 'transactions')}
		>
			<ClipboardList size={23} />
			<span>Transactions</span>
		</button>
		<button
			class:active={activeScreen === 'settings'}
			title="Settings"
			type="button"
			on:click={() => (activeScreen = 'settings')}
		>
			<Settings size={23} />
			<span>Settings</span>
		</button>
	</nav>
</main>

{#if feedbackOpen}
	<div class="feedback-modal-backdrop" role="status" aria-live="polite">
		<div class={`feedback-modal ${feedbackKind}`}>
			<div class="feedback-icon" aria-hidden="true">{feedbackKind === 'success' ? '✓' : '!'}</div>
			<h3>{feedbackTitle}</h3>
			<p>{feedbackMessage}</p>
			<button type="button" on:click={closeFeedback}>OK</button>
		</div>
	</div>
{/if}

<main class="desktop-app">
	<aside class="desktop-sidebar">
		<div class="brand-mark">
			<span>F</span>
			<strong>FinSet</strong>
		</div>
		<nav aria-label="Desktop sections">
			<a href="#dashboard" class:active={desktopScreen === 'dashboard'} on:click|preventDefault={() => (desktopScreen = 'dashboard')}>
				<Home size={18} /> Dashboard
			</a>
			<a href="#transactions" class:active={desktopScreen === 'transactions'} on:click|preventDefault={() => (desktopScreen = 'transactions')}>
				<ClipboardList size={18} /> Transactions
			</a>
			<a href="#wallet" class:active={desktopScreen === 'accounts'} on:click|preventDefault={() => (desktopScreen = 'accounts')}>
				<CreditCard size={18} /> Accounts
			</a>
			<a href="#review" class:active={desktopScreen === 'review'} on:click|preventDefault={() => (desktopScreen = 'review')}>
				<BarChart3 size={18} /> Review
			</a>
			<a href="#settings" class:active={desktopScreen === 'settings'} on:click|preventDefault={() => (desktopScreen = 'settings')}>
				<Settings size={18} /> Settings
			</a>
		</nav>
		<div class="sidebar-footer">
			<button type="button" on:click={() => finance.syncNow()}><RefreshCw size={17} /> Sync</button>
			<span class:offline={!isOnline}>{isOnline ? 'Online' : 'Offline'}</span>
		</div>
	</aside>

	<section class="desktop-main" id="dashboard">
		<header class="desktop-topbar">
			<div>
				<h1>{desktopHeading.title}</h1>
				<p>{desktopHeading.subtitle}</p>
			</div>
				<div class="desktop-actions">
					<button title="Search transactions" type="button" on:click={() => (desktopScreen = 'transactions')}>
						<Search size={20} />
					</button>
					{#if desktopScreen === 'add'}
						<span class="desktop-action-badge">
							<Plus size={18} />
							Add transaction
						</span>
					{:else}
						<button class="add-desktop" type="button" on:click={() => (desktopScreen = 'add')}>
							<Plus size={18} />
							Add transaction
						</button>
					{/if}
				</div>
			</header>

			{#if desktopScreen === 'dashboard'}
			<div class="desktop-filter-row">
				<button type="button"><CalendarDays size={17} /> This month</button>
				<button type="button" on:click={() => (desktopScreen = 'settings')}><Settings size={17} /> Manage widgets</button>
			</div>

		<section class="desktop-kpis">
			<article>
				<span>Household balance</span>
				<strong>{currency(currentBalance)}</strong>
				<small>{allTransactions.length} transactions · {accounts.length} accounts</small>
			</article>
			<article>
				<span>Personal balance</span>
				<strong>{currency(personalBalance)}</strong>
				<small>{personalEntries.length} personal transactions</small>
			</article>
			<article>
				<span>Income</span>
				<strong>{currency(currentSummary?.income ?? 0)}</strong>
				<small>{currentSummary ? readablePeriod(currentSummary.periodKey, grain) : 'No income yet'}</small>
			</article>
			<article>
				<span>Expense</span>
				<strong>{currency(currentSummary?.spent ?? 0)}</strong>
				<small>{categories.length} categories configured</small>
			</article>
		</section>

		<div class="desktop-grid">
			<section class="desktop-card wide-card">
				<div class="desktop-card-head">
					<div>
						<h2>Total balance overview</h2>
						<p>This month compared with recent periods</p>
					</div>
					<AppSelect
						ariaLabel="Desktop report period"
						bind:value={grain}
						options={grainOptions}
						triggerClass="desktop-grain-trigger"
					/>
				</div>
				<div class="line-chart">
					{#each desktopBalanceSeries as summary, index}
						<span
							class:latest={index === latestBalanceSeriesIndex}
							style={`--height:${Math.max(18, Math.min(92, Math.abs(summary.endingBalance || 35) / Math.max(1, Math.abs(currentBalance || 1000)) * 86 + 18))}%;--delay:${index}`}
						></span>
					{/each}
				</div>
				<div class="line-chart-axis">
					{#each desktopBalanceSeries as summary}
						<span>{shortPeriodLabel(summary.periodKey, grain)}</span>
					{/each}
				</div>
				<div class="line-chart-legend">
					<span class="previous">Previous periods</span>
					<span class="current">Current period</span>
				</div>
			</section>

			<section class="desktop-card desktop-stat-card" id="review">
					<div class="desktop-card-head">
						<div>
							<h2>Statistics</h2>
							<p>{reviewEntryType === 'income' ? 'Income by category' : 'Expenses by category'}</p>
						</div>
						<button type="button" on:click={() => (desktopScreen = 'review')}>Details</button>
					</div>
				<div class="desktop-ring">
					<svg class="ring-svg" viewBox="0 0 220 220" aria-label="Desktop category expense ring">
						<circle class="ring-track" cx="110" cy="110" r="90" pathLength="565" />
						{#each statItems as item, index}
							<circle class="ring-segment" cx="110" cy="110" r="90" pathLength="565" style={ringSegmentStyle(statItems, index)} />
						{/each}
					</svg>
					<div>
						<span>{reviewEntryType === 'income' ? 'This month income' : 'This month expense'}</span>
						<strong>{currency(reviewTotal)}</strong>
					</div>
				</div>
				<div class="desktop-legend">
					{#each statItems.slice(0, 5) as item}
						<span style={`--swatch:${item.color}`}>{item.name}</span>
					{/each}
				</div>
			</section>

			<section class="desktop-card wide-card">
					<div class="desktop-card-head">
						<div>
							<h2>Budget and expense comparison</h2>
							<p>Category targets against current spending</p>
						</div>
						<button type="button" on:click={() => (desktopScreen = 'review')}>This year</button>
					</div>
				<div class="bar-chart-legend">
					<span class="spent">Spent</span>
					<span class="target">Target</span>
				</div>
				<div class="bar-chart">
					{#each budgetComparisonItems as item}
						<div>
							<div class="bar-stack" aria-hidden="true">
								<i class="bar-target"></i>
								<i class="bar-spent" style={`--bar:${item.fillPercent}%`}></i>
							</div>
							<span>{item.name}</span>
							<small>{currency(item.spent)} / {currency(item.target)}</small>
						</div>
					{/each}
				</div>
			</section>

			<section class="desktop-card" id="wallet">
				<div class="desktop-card-head">
					<div>
						<h2>Accounts</h2>
						<p>Cards, banks, wallets and cash</p>
					</div>
					<button type="button" on:click={() => (desktopAddAccountWizardOpen = !desktopAddAccountWizardOpen)}>
						<Plus size={16} />
						{desktopAddAccountWizardOpen ? 'Close' : 'Add new'}
					</button>
				</div>
				<div class="desktop-account-list">
					{#each accounts as account}
						<article>
							<div class="entity-lead">
								<span class="entity-icon">{accountEmoji(account)}</span>
								<span style={`--swatch:${account.color}`}></span>
							</div>
							<div>
								<strong>{account.name}</strong>
								<small>{account.type}</small>
							</div>
							<b>{currency(accountBalance(account.id))}</b>
						</article>
					{/each}
				</div>
						{#if desktopAddAccountWizardOpen}
							<form class="desktop-form-grid desktop-inline-wizard" on:submit|preventDefault={submitDesktopAccountWizard}>
								<input name="name" placeholder="Account name" required />
								<div class="field-grid">
									<AppSelect ariaLabel="Account type" bind:value={accountFormType} name="type" options={accountTypeOptions} />
									<input name="openingBalance" type="text" inputmode="decimal" placeholder="Opening" on:input={formatAmountInput} />
								</div>
						<input name="icon" placeholder="Emoji icon (e.g. 🏦)" />
						<input name="color" type="color" value="#2563eb" title="Account color" />
						<div class="desktop-inline-wizard-actions">
							<button type="submit">Add account</button>
							<button class="ghost" type="button" on:click={() => (desktopAddAccountWizardOpen = false)}>Cancel</button>
						</div>
					</form>
				{/if}
			</section>

			<section class="desktop-card" id="categories">
				<div class="desktop-card-head">
					<div>
						<h2>Categories</h2>
						<p>Configured spending buckets</p>
					</div>
					<button type="button" on:click={() => (desktopAddCategoryWizardOpen = !desktopAddCategoryWizardOpen)}>
						<Plus size={16} />
						{desktopAddCategoryWizardOpen ? 'Close' : 'Add new'}
					</button>
				</div>
				<div class="desktop-account-list">
					{#each categories as category}
						<article>
							<div class="entity-lead">
								<span class="entity-icon">{categoryEmoji(category)}</span>
								<span style={`--swatch:${category.color}`}></span>
							</div>
							<div>
								<strong>{category.name}</strong>
								<small>{categoryTypeLabel(category.type)} · Target {currency(category.monthlyTarget)}</small>
							</div>
						</article>
					{/each}
				</div>
						{#if desktopAddCategoryWizardOpen}
							<form class="desktop-form-grid desktop-inline-wizard" on:submit|preventDefault={submitDesktopCategoryWizard}>
								<input name="name" placeholder="Category name" required />
								<div class="field-grid">
									<AppSelect ariaLabel="Category type" bind:value={categoryFormType} name="type" options={categoryTypeOptions} />
									<input name="monthlyTarget" type="text" inputmode="decimal" placeholder="Monthly target" on:input={formatAmountInput} />
								</div>
						<input name="icon" placeholder="Emoji icon (e.g. 🛒)" />
						<input name="color" type="color" value="#10b981" title="Category color" />
						<div class="desktop-inline-wizard-actions">
							<button type="submit">Add category</button>
							<button class="ghost" type="button" on:click={() => (desktopAddCategoryWizardOpen = false)}>Cancel</button>
						</div>
					</form>
				{/if}
			</section>
		</div>

		<section class="desktop-card desktop-transactions" id="transactions">
			<div class="desktop-card-head">
				<div>
					<h2>Recent transactions</h2>
					<p>Latest shared group activity</p>
				</div>
				<button type="button" on:click={() => (desktopScreen = 'transactions')}>Open full table</button>
			</div>
			<div class="desktop-table">
				<div class="desktop-table-head">
					<span>Date</span>
					<span>Merchant</span>
					<span>Category</span>
					<span>Account</span>
					<span>Amount</span>
				</div>
				{#each filteredTransactions.slice(0, 8) as entry}
					<article>
						<time>{formatDate(entry.occurredOn)}</time>
						<strong>{entry.merchant}</strong>
						<span>{categoryName(entry.categoryId)}</span>
						<span>{accountName(entry.accountId)}</span>
						<b class:negative={entry.type === 'expense'}>{entry.type === 'expense' ? '-' : '+'}{currency(entry.amount)}</b>
					</article>
				{:else}
					<p class="muted">{transactionSearchQueryNormalized ? 'No matching transactions.' : 'No transactions yet.'}</p>
				{/each}
			</div>
		</section>

		<section class="desktop-card desktop-settings" id="settings">
			<div class="desktop-card-head">
				<div>
					<h2>Settings and configuration</h2>
					<p>Use the mobile settings tab for full account/category editing. Server sync: {$syncStatus.message}</p>
				</div>
				<button type="button" on:click={() => (desktopScreen = 'settings')}>Open settings</button>
			</div>
		</section>
			{:else if desktopScreen === 'transactions'}
				<section class="desktop-card desktop-transactions desktop-page-card">
					<div class="desktop-card-head">
						<div>
							<h2>Transactions</h2>
							<p>All shared ledger entries in one table.</p>
						</div>
						<button type="button" on:click={() => (desktopScreen = 'add')}><Plus size={17} /> Add transaction</button>
					</div>
					<div class="desktop-table">
						<div class="desktop-table-head">
							<span>Date</span>
							<span>Merchant</span>
							<span>Category</span>
							<span>Account</span>
							<span>Amount</span>
						</div>
						{#each filteredTransactions as entry}
							<article>
								<time>{formatDate(entry.occurredOn)}</time>
								<strong>{entry.merchant}</strong>
								<span>{categoryName(entry.categoryId)}</span>
								<span>{accountName(entry.accountId)}</span>
								<b class:negative={entry.type === 'expense'}>{entry.type === 'expense' ? '-' : '+'}{currency(entry.amount)}</b>
							</article>
						{:else}
							<p class="muted">{transactionSearchQueryNormalized ? 'No matching transactions.' : 'No transactions yet.'}</p>
						{/each}
					</div>
				</section>
			{:else if desktopScreen === 'accounts'}
				<div class="desktop-page-grid">
					<section class="desktop-card">
						<div class="desktop-card-head">
							<div>
								<h2>Accounts</h2>
								<p>Balances by cash, bank, card, and wallet.</p>
							</div>
							<button type="button" on:click={() => (desktopAddAccountWizardOpen = !desktopAddAccountWizardOpen)}>
								<Plus size={16} />
								{desktopAddAccountWizardOpen ? 'Close' : 'Add new'}
							</button>
						</div>
						<div class="desktop-account-list">
							{#each accounts as account}
								<article>
									<div class="entity-lead">
										<span class="entity-icon">{accountEmoji(account)}</span>
										<span style={`--swatch:${account.color}`}></span>
									</div>
									<div>
										<strong>{account.name}</strong>
										<small>{account.type}</small>
									</div>
									<b>{currency(accountBalance(account.id))}</b>
								</article>
							{/each}
						</div>
						{#if desktopAddAccountWizardOpen}
							<form class="desktop-form-grid desktop-inline-wizard" on:submit|preventDefault={submitDesktopAccountWizard}>
								<input name="name" placeholder="Account name" required />
								<div class="field-grid">
									<AppSelect ariaLabel="Account type" bind:value={accountFormType} name="type" options={accountTypeOptions} />
									<input name="openingBalance" type="text" inputmode="decimal" placeholder="Opening" on:input={formatAmountInput} />
								</div>
								<input name="icon" placeholder="Emoji icon (e.g. 🏦)" />
								<input name="color" type="color" value="#2563eb" title="Account color" />
								<div class="desktop-inline-wizard-actions">
									<button type="submit">Add account</button>
									<button class="ghost" type="button" on:click={() => (desktopAddAccountWizardOpen = false)}>Cancel</button>
								</div>
							</form>
						{/if}
					</section>
					<section class="desktop-card">
						<div class="desktop-card-head">
							<div>
								<h2>Categories</h2>
								<p>Configured spending buckets.</p>
							</div>
							<button type="button" on:click={() => (desktopAddCategoryWizardOpen = !desktopAddCategoryWizardOpen)}>
								<Plus size={16} />
								{desktopAddCategoryWizardOpen ? 'Close' : 'Add new'}
							</button>
						</div>
						<div class="desktop-account-list">
							{#each categories as category}
								<article>
									<div class="entity-lead">
										<span class="entity-icon">{categoryEmoji(category)}</span>
										<span style={`--swatch:${category.color}`}></span>
									</div>
									<div>
										<strong>{category.name}</strong>
										<small>{categoryTypeLabel(category.type)} · Target {currency(category.monthlyTarget)}</small>
									</div>
								</article>
							{/each}
						</div>
						{#if desktopAddCategoryWizardOpen}
							<form class="desktop-form-grid desktop-inline-wizard" on:submit|preventDefault={submitDesktopCategoryWizard}>
								<input name="name" placeholder="Category name" required />
								<div class="field-grid">
									<AppSelect ariaLabel="Category type" bind:value={categoryFormType} name="type" options={categoryTypeOptions} />
									<input name="monthlyTarget" type="text" inputmode="decimal" placeholder="Monthly target" on:input={formatAmountInput} />
								</div>
								<input name="icon" placeholder="Emoji icon (e.g. 🛒)" />
								<input name="color" type="color" value="#10b981" title="Category color" />
								<div class="desktop-inline-wizard-actions">
									<button type="submit">Add category</button>
									<button class="ghost" type="button" on:click={() => (desktopAddCategoryWizardOpen = false)}>Cancel</button>
								</div>
							</form>
						{/if}
					</section>
				</div>
			{:else if desktopScreen === 'review'}
				<div class="desktop-page-grid review-page">
					<section class="desktop-card desktop-stat-card">
						<div class="desktop-card-head">
							<div>
								<h2>Review</h2>
								<p>{reviewEntryType === 'income' ? 'Category income by selected period.' : 'Category spending by selected period.'}</p>
							</div>
							<AppSelect
								ariaLabel="Review period"
								bind:value={grain}
								options={grainOptions}
								triggerClass="desktop-grain-trigger"
							/>
						</div>
						<div class="desktop-ring">
							<svg class="ring-svg" viewBox="0 0 220 220" aria-label="Category expense ring">
								<circle class="ring-track" cx="110" cy="110" r="90" pathLength="565" />
								{#each statItems as item, index}
									<circle class="ring-segment" cx="110" cy="110" r="90" pathLength="565" style={ringSegmentStyle(statItems, index)} />
								{/each}
							</svg>
							<div>
								<span>{currentSummary ? readablePeriod(currentSummary.periodKey, grain) : 'No period yet'}</span>
								<strong>{currency(reviewTotal)}</strong>
							</div>
						</div>
						<div class="desktop-legend">
							{#each statItems as item}
								<span style={`--swatch:${item.color}`}>{item.name}: {currency(item.amount)}</span>
							{/each}
						</div>
					</section>
					<section class="desktop-card">
						<div class="desktop-card-head">
							<div>
								<h2>Period history</h2>
								<p>Ending balance and net movement.</p>
							</div>
						</div>
						<div class="desktop-account-list">
							{#each summaries as summary}
								<article>
									<div>
										<strong>{readablePeriod(summary.periodKey, grain)}</strong>
										<small>Ending {currency(summary.endingBalance)}</small>
									</div>
									<b>{formatSignedCurrency(summary.netCashFlow)}</b>
								</article>
							{:else}
								<p class="muted">No periods yet.</p>
							{/each}
						</div>
					</section>
				</div>
			{:else if desktopScreen === 'add'}
				<section class="desktop-card desktop-page-card">
					<div class="desktop-card-head">
						<div>
							<h2>Add transaction</h2>
							<p>Create an expense or income entry.</p>
						</div>
					</div>
					<form class="desktop-form-grid" on:submit|preventDefault={(event) => submitMovement(event, 'desktop')}>
						<Tabs.Root bind:value={selectedEntryType} class="entry-type-tabs-root">
							<Tabs.List class="entry-type-tabs" aria-label="Entry type" style={`--active-index:${entryTypeTabIndex}`}>
								<Tabs.Trigger value="expense" class="entry-type-tab">Expense</Tabs.Trigger>
								<Tabs.Trigger value="income" class="entry-type-tab">Income</Tabs.Trigger>
							</Tabs.List>
						</Tabs.Root>
						<input name="type" type="hidden" value={selectedEntryType} />
						<label>
							Amount
							<input name="amount" type="text" inputmode="decimal" placeholder="0.00" on:input={formatAmountInput} required />
						</label>
						<label class="merchant-field">
							Merchant or source
							<input
								bind:value={merchantQuery}
								autocomplete="off"
								name="merchant"
								on:blur={handleMerchantBlur}
								on:focus={handleMerchantFocus}
								on:input={handleMerchantInput}
								placeholder="Supermarket, salary, transfer..."
								required
							/>
							{#if showMerchantSuggestions}
								<div class="merchant-suggestions" role="listbox" aria-label="Merchant suggestions">
									{#each merchantSuggestions as merchantName}
										<button type="button" on:mousedown|preventDefault={() => applyMerchantSuggestion(merchantName)}>
											{merchantName}
										</button>
									{/each}
								</div>
							{/if}
						</label>
						<div class="field-grid">
							<label>
								Account
								<AppSelect
									ariaLabel="Account"
									bind:value={selectedAccountId}
									disabled={accountSelectOptions.length === 0}
									name="accountId"
									options={accountSelectOptions}
									placeholder="No accounts configured"
									required
								/>
							</label>
							<label>
								Category
								<AppSelect
									ariaLabel="Category"
									bind:value={selectedCategoryId}
									disabled={categorySelectOptions.length === 0}
									name="categoryId"
									options={categorySelectOptions}
									placeholder="No categories configured"
									required
								/>
							</label>
						</div>
						<div class="field-grid">
							<label>
								Date
								<DatePicker.Root bind:value={selectedDate} weekdayFormat="short" fixedWeeks={true}>
									<div class="date-picker-field">
										<DatePicker.Input name="occurredOn" class="date-picker-input">
											{#snippet children({ segments })}
												{#each segments as segment, index (`desktop-${segment.part}-${index}`)}
													{#if segment.part === 'literal'}
														<span class="date-picker-literal">{segment.value}</span>
													{:else}
														<DatePicker.Segment part={segment.part} class="date-picker-segment">
															{segment.value}
														</DatePicker.Segment>
													{/if}
												{/each}
											{/snippet}
										</DatePicker.Input>
										<DatePicker.Trigger class="date-picker-trigger" aria-label="Open calendar">
											<CalendarDays size={18} />
										</DatePicker.Trigger>
									</div>
									<DatePicker.Portal>
										<DatePicker.Content class="date-picker-content" sideOffset={8} align="end">
											<DatePicker.Calendar class="date-picker-calendar">
												{#snippet children({ months, weekdays })}
													<DatePicker.Header class="date-picker-calendar-header">
														<DatePicker.PrevButton class="date-picker-nav-button" aria-label="Previous month">‹</DatePicker.PrevButton>
														<DatePicker.Heading class="date-picker-heading" />
														<DatePicker.NextButton class="date-picker-nav-button" aria-label="Next month">›</DatePicker.NextButton>
													</DatePicker.Header>
													{#each months as month}
														<DatePicker.Grid class="date-picker-grid">
															<DatePicker.GridHead>
																<DatePicker.GridRow class="date-picker-grid-row">
																	{#each weekdays as day}
																		<DatePicker.HeadCell class="date-picker-head-cell">{day}</DatePicker.HeadCell>
																	{/each}
																</DatePicker.GridRow>
															</DatePicker.GridHead>
															<DatePicker.GridBody>
																{#each month.weeks as weekDates}
																	<DatePicker.GridRow class="date-picker-grid-row">
																		{#each weekDates as date}
																			<DatePicker.Cell {date} month={month.value}>
																				<DatePicker.Day class="date-picker-day" />
																			</DatePicker.Cell>
																		{/each}
																	</DatePicker.GridRow>
																{/each}
															</DatePicker.GridBody>
														</DatePicker.Grid>
													{/each}
												{/snippet}
											</DatePicker.Calendar>
										</DatePicker.Content>
									</DatePicker.Portal>
								</DatePicker.Root>
							</label>
							<label>
								Note
								<input name="note" placeholder="Optional" />
							</label>
						</div>
						<button class="primary-button" type="submit" disabled={!accounts.length || !addCategoryOptions.length}>
							<span class="sr-only">Save movement</span>
							<Plus size={20} aria-hidden="true" />
						</button>
					</form>
				</section>
			{:else if desktopScreen === 'settings'}
				<section class="desktop-card desktop-page-card">
					<div class="desktop-card-head">
						<div>
							<h2>Group and sync</h2>
							<p>{$syncStatus.message}</p>
						</div>
					</div>
					<section class="desktop-theme-toggle">
						<h3>Appearance</h3>
						<p class="muted">Current mode: {themeMode === 'dark' ? 'Dark' : 'Light'}</p>
						<button class="theme-toggle-button" type="button" on:click={toggleTheme}>
							{#if themeMode === 'dark'}
								<Sun size={16} />
								Switch to light mode
							{:else}
								<Moon size={16} />
								Switch to dark mode
							{/if}
						</button>
					</section>
					<form class="desktop-form-grid" on:submit|preventDefault={(event) => submitAndSync(finance.updateGroupName, event)}>
						<input name="name" value={activeGroup?.name ?? ''} placeholder="Group name" />
						<p class="muted">Invite code {activeGroup?.inviteCode ?? 'LOCAL'}</p>
						<button type="submit">Rename group</button>
					</form>
					<div class="button-row">
						<button type="button" on:click={() => finance.syncNow()}><RefreshCw size={16} /> Sync now</button>
						<button class="ghost" type="button" on:click={logout}>Logout</button>
					</div>
					<form class="desktop-form-grid" on:submit|preventDefault={handleCreateApiKey}>
						<h2>API keys</h2>
						<p class="muted">Create API keys for server-to-server or script access.</p>
						<input name="name" placeholder="API key name" required />
						<button type="submit" disabled={generatingApiKey}>{generatingApiKey ? 'Generating...' : 'Generate API key'}</button>
						{#if apiKeyFormError}
							<p class="auth-error">{apiKeyFormError}</p>
						{/if}
						{#if generatedApiKey}
							<div class="generated-api-key">
								<p>Copy now (shown once)</p>
								<input value={generatedApiKey.key} readonly />
								<button type="button" on:click={copyGeneratedApiKey}>Copy key</button>
							</div>
						{/if}
					</form>
				</section>
			{/if}
		</section>
	</main>

<script lang="ts">
	import {
		ArrowLeft,
		BarChart3,
		ChevronDown,
		ClipboardList,
		CreditCard,
		FileScan,
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
	import { Collapsible, DatePicker, Meter, Tabs } from 'bits-ui';
	import { parseDate, type DateValue } from '@internationalized/date';
	import AppSelect, { consumeSelectClickThroughGuard } from '$lib/components/AppSelect.svelte';
	import StatementIngestion from '$lib/components/StatementIngestion.svelte';
	import { onMount, tick } from 'svelte';
	import { slide } from 'svelte/transition';
	import { buildPeriodSummaries, getCurrentBalance, periodKey, readablePeriod } from '$lib/reporting';
	import { finance } from '$lib/finance';
	import { clearSession, generateApiKey, getStoredSession, login, register, type APIKeyData, type AuthSession } from '$lib/auth';
	import { patchSettings } from '$lib/db';
	import { deleteTransactionRemote, searchTransactionsRemote, updateTransactionRemote } from '$lib/transactions';
	import type { CategoryScope, CategoryType, LedgerEntry, PeriodCategoryTotal, PeriodGrain, PeriodSummary } from '$lib/types';
	import {
		cents,
		currency as formatCurrency,
		entryAmountInBaseCurrency,
		formatSignedCurrency,
		isActive,
		normalizeCurrencyCode,
		todayInputValue
	} from '$lib/utils';

	type Screen = 'home' | 'review' | 'add' | 'transactions' | 'search' | 'transactionDetail' | 'settings';
	type DesktopScreen = 'dashboard' | 'transactions' | 'transactionDetail' | 'accounts' | 'review' | 'statements' | 'settings' | 'add';
	type SettingsSubpage = 'overview' | 'accounts' | 'accountEdit' | 'categories' | 'categoryEdit' | 'adjustments' | 'statements';
	type StatItem = { name: string; color: string; amount: number };
	type BudgetComparisonItem = { id: string; name: string; color: string; spent: number; target: number; fillPercent: number };
	type FeedbackKind = 'success' | 'error';
	type SelectOption = { value: string; label: string; disabled?: boolean };
	type HomeBalancePeriod = 'today' | 'month';
	type TransactionPeriodMode = 'month' | 'year';
	type TransactionViewFilter = 'all' | 'household' | 'personal' | 'income' | 'expense';

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
	let desktopTransactionDetailOrigin: 'dashboard' | 'transactions' = 'transactions';
	let desktopSearchOpen = false;
	let desktopSearchInput: HTMLInputElement | null = null;
	let homeBalancePeriod: HomeBalancePeriod = 'month';
	let transactionPeriodMode: TransactionPeriodMode = 'month';
	let selectedTransactionMonth = todayInputValue().slice(0, 7);
	let selectedTransactionYear = todayInputValue().slice(0, 4);
	let transactionViewFilter: TransactionViewFilter = 'all';
	let desktopAddAccountWizardOpen = false;
	let desktopAddCategoryWizardOpen = false;
	let transactionSearchQuery = '';
	let transactionSearchMonthsBack = '6';
	let transactionSearchOrigin: Screen = 'transactions';
	let mobileTransactionCategoryId = '';
	let mobileTransactionPeriodKey = todayInputValue().slice(0, 7);
	let mobileTransactionGrain: PeriodGrain = 'month';
	let mobileTransactionViewFilter: TransactionViewFilter = 'all';
	let mobileTransactionsScrollTop = 0;
	let lastMobileTransactionPeriodKey = mobileTransactionPeriodKey;
	let restoreMobileTransactionsScroll = false;
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
	let selectedReviewPeriodKey = '';
	let reviewPeriodRailEl: HTMLElement | null = null;
	let selectedDate: DateValue = parseDate(todayInputValue());
	let selectedAccountId = '';
	let selectedCategoryId = '';
	let selectedCurrency = 'SGD';
	let transactionEditMode = false;
	let transactionEditType: 'expense' | 'income' = 'expense';
	let transactionEditAccountId = '';
	let transactionEditCategoryId = '';
	let transactionEditCurrency = 'SGD';
	let transactionEditAmount = '';
	let transactionEditDate = todayInputValue();
	let transactionEditDateValue: DateValue = parseDate(todayInputValue());
	let transactionEditMerchant = '';
	let transactionEditNote = '';
	let transactionDeleteConfirmOpen = false;
	let transactionDeleting = false;
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
	let categoryFormScope: CategoryScope = 'household';
	let themeMode: 'light' | 'dark' = 'light';
	let settingsSubpage: SettingsSubpage = 'overview';
	let selectedSettingsAccountId = '';
	let selectedSettingsCategoryId = '';
	let selectedSettingsAccountType: 'cash' | 'bank' | 'card' | 'wallet' = 'bank';
	let selectedSettingsCategoryType: CategoryType = 'expense';
	let selectedSettingsCategoryScope: CategoryScope = 'household';
	let homeScreenEl: HTMLElement | null = null;
	let transactionsScreenEl: HTMLElement | null = null;
	let pullRefreshing = false;
	let pullRefreshScreen: 'home' | 'transactions' | null = null;
	let pullDistance = 0;
	let pullStartY = 0;
	let pullTracking = false;
	const pullTriggerDistance = 68;
	const pullMaxDistance = 104;
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
	const categoryScopeOptions: SelectOption[] = [
		{ value: 'household', label: 'Household' },
		{ value: 'user', label: 'User' }
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
	const transactionPeriodModeOptions: SelectOption[] = [
		{ value: 'month', label: 'Month' },
		{ value: 'year', label: 'Year' }
	];
	const transactionViewFilterOptions: SelectOption[] = [
		{ value: 'all', label: 'All transactions' },
		{ value: 'household', label: 'Household' },
		{ value: 'personal', label: 'Personal' },
		{ value: 'income', label: 'Income' },
		{ value: 'expense', label: 'Expense' }
	];
	const currencyOptions: SelectOption[] = [
		'SGD',
		'USD',
		'EUR',
		'GBP',
		'JPY',
		'AUD',
		'CAD',
		'CHF',
		'CNY',
		'HKD',
		'IDR',
		'MYR',
		'THB',
		'PHP',
		'KRW',
		'NZD'
	].map((value) => ({ value, label: value }));

	$: state = $finance;
	$: baseCurrency = normalizeCurrencyCode(state?.settings.baseCurrency ?? 'SGD');
	$: activeGroup = state?.groups.find((group) => group.id === state.settings.activeGroupId);
	$: allAccounts =
		state?.accounts
			.filter((account) => account.groupId === state.settings.activeGroupId)
			.map((account, index) => ({ ...account, color: displayColor(account.color, index) })) ?? [];
	$: accounts = allAccounts.filter(isActive);
	$: allCategories =
		state?.categories
			.filter((category) => category.groupId === state.settings.activeGroupId)
			.map((category, index) => ({
				...category,
				type: normalizeCategoryType(category.type, category.name),
				scope: normalizeCategoryScope(category.scope),
				ownerUserId: category.ownerUserId ?? null,
				color: displayColor(category.color, index + 1)
			}))
			.filter((category) => isCategoryVisibleToUser(category, state?.settings.deviceUserId ?? '')) ?? [];
	$: categories = allCategories.filter(isActive);
	$: entries = state?.entries.filter((entry) => entry.groupId === state.settings.activeGroupId && isActive(entry)) ?? [];
	$: accountUsageCounts = countTransactionUsage(entries, 'accountId');
	$: categoryUsageCounts = countTransactionUsage(entries, 'categoryId');
	$: addCategoryOptions = sortByTransactionUsage(
		getEntryCategoryOptions(categories, selectedEntryType),
		categoryUsageCounts
	);
	$: accountSelectOptions = [
		...sortByTransactionUsage(accounts, accountUsageCounts),
		...sortByTransactionUsage(
			allAccounts.filter((account) => !isActive(account)),
			accountUsageCounts
		)
	].map((account) => ({
		value: account.id,
		label: isActive(account) ? account.name : `${account.name} (Inactive)`,
		disabled: !isActive(account)
	}));
	$: categorySelectOptions = [
		...addCategoryOptions,
		...sortByTransactionUsage(
			getEntryCategoryOptions(
				allCategories.filter((category) => !isActive(category)),
				selectedEntryType
			),
			categoryUsageCounts
		)
	].map((category) => ({
		value: category.id,
		label: `${category.name} (${categoryScopeLabel(category.scope)})${isActive(category) ? '' : ' · Inactive'}`,
		disabled: !isActive(category)
	}));
	$: transactionEditCategoryOptions = [
		...sortByTransactionUsage(getEntryCategoryOptions(categories, transactionEditType), categoryUsageCounts),
		...sortByTransactionUsage(
			getEntryCategoryOptions(
				allCategories.filter((category) => !isActive(category)),
				transactionEditType
			),
			categoryUsageCounts
		)
	].map((category) => ({
		value: category.id,
		label: `${category.name} (${categoryScopeLabel(category.scope)})${isActive(category) ? '' : ' · Inactive'}`,
		disabled: !isActive(category)
	}));
	$: settingsCategoryOptions = categories.map((category) => ({
		value: category.id,
		label: `${category.name} (${categoryScopeLabel(category.scope)})`
	}));
	$: {
		if (settingsCategoryOptions.length === 0) {
			adjustmentCategoryId = '';
		} else if (!settingsCategoryOptions.some((option) => option.value === adjustmentCategoryId)) {
			adjustmentCategoryId = settingsCategoryOptions[0].value;
		}
	}
	$: {
		const selectableAccountOptions = accountSelectOptions.filter((option) => !option.disabled);
		if (selectableAccountOptions.length === 0) {
			selectedAccountId = '';
		} else if (!selectableAccountOptions.some((option) => option.value === selectedAccountId)) {
			selectedAccountId = selectableAccountOptions[0].value;
		}
	}
	$: {
		if (addCategoryOptions.length === 0) {
			selectedCategoryId = '';
		} else if (!addCategoryOptions.some((category) => category.id === selectedCategoryId)) {
			selectedCategoryId = addCategoryOptions[0].id;
		}
	}
	$: {
		if (transactionEditCategoryOptions.length === 0) {
			transactionEditCategoryId = '';
		} else if (!transactionEditCategoryOptions.some((category) => category.value === transactionEditCategoryId)) {
			transactionEditCategoryId = transactionEditCategoryOptions[0].value;
		}
	}
	$: adjustments =
		state?.adjustments.filter((adjustment) => adjustment.groupId === state.settings.activeGroupId && isActive(adjustment)) ??
		[];
	$: merchants = state?.merchants.filter((merchant) => merchant.groupId === state.settings.activeGroupId && isActive(merchant)) ?? [];
	$: categoryById = new Map(allCategories.map((category) => [category.id, category]));
	$: summaries = buildPeriodSummaries(accounts, allCategories, entries, adjustments, grain, baseCurrency);
	$: currentSummary = summaries[0];
	$: householdEntries = entries.filter((entry) => {
		const category = categoryById.get(entry.categoryId);
		return normalizeCategoryScope(category?.scope) === 'household';
	});
	$: currentBalance = getCurrentBalance(accounts, householdEntries, baseCurrency);
	$: currentMonthKey = todayInputValue().slice(0, 7);
	$: currentMonthEntries = entries.filter((entry) => entry.occurredOn.startsWith(currentMonthKey));
	$: homeBalancePeriodLabel = homeBalancePeriod === 'today' ? 'Today' : 'This month';
	$: homeBalanceEntries =
		homeBalancePeriod === 'today'
			? entries.filter((entry) => entry.occurredOn === todayInputValue())
			: currentMonthEntries;
	$: homeIncomeEntries = homeBalanceEntries.filter((entry) => entry.type === 'income');
	$: homeExpenseEntries = homeBalanceEntries.filter((entry) => entry.type === 'expense');
	$: homeIncome = homeIncomeEntries.reduce((sum, entry) => sum + entryAmountInBaseCurrency(entry, baseCurrency), 0);
	$: homeSpent = homeExpenseEntries.reduce((sum, entry) => sum + entryAmountInBaseCurrency(entry, baseCurrency), 0);
	$: currentMonthCategoryAmounts = currentMonthEntries.reduce((totals, entry) => {
		totals.set(entry.categoryId, (totals.get(entry.categoryId) ?? 0) + entryAmountInBaseCurrency(entry, baseCurrency));
		return totals;
	}, new Map<string, number>());
	$: homeHouseholdEntries = homeBalanceEntries.filter((entry) => {
		const category = categoryById.get(entry.categoryId);
		return normalizeCategoryScope(category?.scope) === 'household';
	});
	$: homeHouseholdBalance = homeHouseholdEntries.reduce(
		(sum, entry) =>
			sum +
			(entry.type === 'income'
				? entryAmountInBaseCurrency(entry, baseCurrency)
				: -entryAmountInBaseCurrency(entry, baseCurrency)),
		0
	);
	$: personalEntries = entries.filter((entry) => {
		const category = categoryById.get(entry.categoryId);
		if (normalizeCategoryScope(category?.scope) === 'user') {
			if (!category?.ownerUserId) return true;
			return category.ownerUserId === state?.settings.deviceUserId;
		}
		return false;
	});
	$: homePersonalEntries = personalEntries.filter((entry) =>
		homeBalancePeriod === 'today' ? entry.occurredOn === todayInputValue() : entry.occurredOn.startsWith(currentMonthKey)
	);
	$: homePersonalIncome = homePersonalEntries
		.filter((entry) => entry.type === 'income')
		.reduce((sum, entry) => sum + entryAmountInBaseCurrency(entry, baseCurrency), 0);
	$: homePersonalSpent = homePersonalEntries
		.filter((entry) => entry.type === 'expense')
		.reduce((sum, entry) => sum + entryAmountInBaseCurrency(entry, baseCurrency), 0);
	$: homePersonalBalance = homePersonalIncome - homePersonalSpent;
	$: recentEntries = [...entries].sort((a, b) => b.occurredOn.localeCompare(a.occurredOn)).slice(0, 5);
	$: allTransactions = [...entries].sort((a, b) => `${b.occurredOn}${b.createdAt}`.localeCompare(`${a.occurredOn}${a.createdAt}`));
	$: transactionMonthOptions = buildTransactionMonthOptions(allTransactions);
	$: transactionYearOptions = buildTransactionYearOptions(allTransactions);
	$: selectedTransactionPeriodLabel =
		transactionPeriodMode === 'month' ? formatTransactionMonthLabel(selectedTransactionMonth) : selectedTransactionYear;
	$: desktopPeriodTransactions = filterTransactionsByPeriod(
		allTransactions,
		transactionPeriodMode,
		selectedTransactionMonth,
		selectedTransactionYear
	);
	$: desktopViewTransactions = filterTransactionsByView(desktopPeriodTransactions, transactionViewFilter);
	$: selectedTransactionViewLabel =
		transactionViewFilterOptions.find((option) => option.value === transactionViewFilter)?.label ?? 'All transactions';
	$: selectedTransaction =
		allTransactions.find((entry) => entry.id === selectedTransactionId) ??
		(selectedTransactionFallback?.id === selectedTransactionId ? selectedTransactionFallback : null);
	$: transactionSearchQueryNormalized = normalizeMerchantText(transactionSearchQuery);
	$: filteredTransactions = filterTransactions(allTransactions, transactionSearchQueryNormalized, transactionSearchMonthsBack);
	$: desktopFilteredTransactions = filterTransactions(desktopViewTransactions, transactionSearchQueryNormalized, 'all');
	$: mobilePeriodTransactions = mobileTransactionPeriodKey
		? allTransactions.filter(
				(entry) =>
					(!mobileTransactionCategoryId || entry.categoryId === mobileTransactionCategoryId) &&
					periodKey(entry.occurredOn, mobileTransactionGrain) === mobileTransactionPeriodKey
			)
		: allTransactions;
	$: mobileTransactions = filterTransactionsByView(mobilePeriodTransactions, mobileTransactionViewFilter);
	$: mobileTransactionCategory = allCategories.find((category) => category.id === mobileTransactionCategoryId);
	$: mobileTransactionPeriodLabel = mobileTransactionPeriodKey
		? readablePeriod(mobileTransactionPeriodKey, mobileTransactionGrain)
		: 'All time';
	$: mobileTransactionFilterLabel =
		mobileTransactionCategory ? `${mobileTransactionCategory.name} · ${mobileTransactionPeriodLabel}` : mobileTransactionPeriodLabel;
	$: selectedMobileTransactionViewLabel =
		transactionViewFilterOptions.find((option) => option.value === mobileTransactionViewFilter)?.label ?? 'All transactions';
	$: mobileTransactionEmptyLabel =
		mobileTransactionViewFilter === 'all' ? 'transactions' : selectedMobileTransactionViewLabel.toLowerCase();
	$: if (mobileTransactionPeriodKey !== lastMobileTransactionPeriodKey) {
		lastMobileTransactionPeriodKey = mobileTransactionPeriodKey;
		if (activeScreen === 'transactions') {
			void (async () => {
				await tick();
				if (transactionsScreenEl) transactionsScreenEl.scrollTop = 0;
			})();
		}
	}
	$: searchPageResults = remoteSearchResults ?? filteredTransactions;
	$: topCategories = (currentSummary?.categories ?? []).filter((item) => Math.abs(item.net) > 0).slice(0, 4);
	$: homeTiles = topCategories.length
		? topCategories.map((item) => ({
				id: item.categoryId,
				name: item.categoryName,
				color: item.categoryColor,
				amount: Math.abs(item.net)
			}))
		: categories.slice(0, 2).map((item) => ({
				id: item.id,
				name: item.name,
				color: item.color,
				amount: item.monthlyTarget
			}));
	$: reviewPeriodSummaries = [...summaries].reverse();
	$: {
		if (!summaries.some((summary) => summary.periodKey === selectedReviewPeriodKey)) {
			selectedReviewPeriodKey = summaries[0]?.periodKey ?? periodKey(todayInputValue(), grain);
		}
	}
	$: selectedReviewSummary = summaries.find((summary) => summary.periodKey === selectedReviewPeriodKey);
	$: if (reviewPeriodRailEl && selectedReviewPeriodKey) {
		void centerSelectedReviewPeriod(reviewPeriodRailEl, selectedReviewPeriodKey);
	}
	$: statItems = buildStatItems(currentSummary?.categories ?? [], reviewEntryType);
	$: reviewStatItems = buildStatItems(selectedReviewSummary?.categories ?? [], reviewEntryType);
	$: reviewTotal =
		reviewEntryType === 'income'
			? (selectedReviewSummary?.income ?? reviewStatItems.reduce((sum, item) => sum + item.amount, 0))
			: (selectedReviewSummary?.spent ?? reviewStatItems.reduce((sum, item) => sum + item.amount, 0));
	$: desktopReviewTotal =
		reviewEntryType === 'income'
			? (currentSummary?.income ?? statItems.reduce((sum, item) => sum + item.amount, 0))
			: (currentSummary?.spent ?? statItems.reduce((sum, item) => sum + item.amount, 0));
	$: reviewTotalLabel = reviewEntryType === 'income' ? 'Total received' : 'Total spent';
	$: reviewTopTransactions = getReviewTopTransactions(entries, selectedReviewSummary?.periodKey, grain, reviewEntryType);
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
	$: selectedSettingsAccount = allAccounts.find((account) => account.id === selectedSettingsAccountId) ?? null;
	$: selectedSettingsCategory = allCategories.find((category) => category.id === selectedSettingsCategoryId) ?? null;
	$: if (selectedSettingsAccount) {
		selectedSettingsAccountType = selectedSettingsAccount.type;
	}
	$: if (selectedSettingsCategory) {
		selectedSettingsCategoryType = normalizeCategoryType(selectedSettingsCategory.type, selectedSettingsCategory.name);
		selectedSettingsCategoryScope = normalizeCategoryScope(selectedSettingsCategory.scope);
	}
	$: desktopSpendingSeries = summaries.slice(0, 8).reverse();
	$: desktopSpendingSeriesMax = Math.max(1, ...desktopSpendingSeries.map((summary) => summary.spent));
	$: desktopSpendingCategoryLegend = buildSpendingCategoryLegend(desktopSpendingSeries);
	$: latestSpendingSeriesIndex = Math.max(0, desktopSpendingSeries.length - 1);
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
			isOnline = navigator.onLine;
			const storedTheme = window.localStorage.getItem('spendit-theme');
			if (storedTheme === 'dark' || storedTheme === 'light') {
				themeMode = storedTheme;
			}
			await finance.init();
			if (authSession?.user?.id) {
				await patchSettings({ deviceUserId: authSession.user.id });
				await finance.init();
				if (isOnline) {
					await finance.syncNow();
				}
			}
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

	function handlePullStart(screen: 'home' | 'transactions', event: TouchEvent): void {
		if (pullRefreshing || activeScreen !== screen) return;
		const container = screen === 'home' ? homeScreenEl : transactionsScreenEl;
		if (!container || container.scrollTop > 0) return;
		const touch = event.touches[0];
		if (!touch) return;
		pullTracking = true;
		pullRefreshScreen = screen;
		pullStartY = touch.clientY;
		pullDistance = 0;
	}

	function handlePullMove(screen: 'home' | 'transactions', event: TouchEvent): void {
		if (!pullTracking || pullRefreshScreen !== screen || pullRefreshing || activeScreen !== screen) return;
		const container = screen === 'home' ? homeScreenEl : transactionsScreenEl;
		const touch = event.touches[0];
		if (!container || !touch) return;
		if (container.scrollTop > 0) {
			cancelPullTracking();
			return;
		}

		const delta = touch.clientY - pullStartY;
		if (delta <= 0) {
			pullDistance = 0;
			return;
		}

		pullDistance = Math.min(pullMaxDistance, delta * 0.45);
	}

	function handlePullEnd(screen: 'home' | 'transactions'): void {
		if (!pullTracking || pullRefreshScreen !== screen || pullRefreshing || activeScreen !== screen) {
			cancelPullTracking();
			return;
		}
		const shouldRefresh = pullDistance >= pullTriggerDistance;
		cancelPullTracking();
		if (shouldRefresh) {
			void pullToRefresh(screen);
		}
	}

	function cancelPullTracking(): void {
		pullTracking = false;
		pullRefreshScreen = null;
		pullDistance = 0;
	}

	async function pullToRefresh(screen: 'home' | 'transactions'): Promise<void> {
		if (pullRefreshing) return;
		pullRefreshing = true;
		pullRefreshScreen = screen;
		pullDistance = pullTriggerDistance;

		try {
			await refreshFromBackend(screen);
		} finally {
			setTimeout(() => {
				pullRefreshing = false;
				pullRefreshScreen = null;
				pullDistance = 0;
			}, 180);
		}
	}

	async function refreshFromBackend(screen: 'home' | 'transactions'): Promise<void> {
		if (!isOnline) {
			showFeedback('error', 'Offline', 'You are offline. Showing local data.');
			return;
		}
		if (!authSession) {
			showFeedback('error', 'Not signed in', 'Login to sync with backend.');
			return;
		}

		const synced = await finance.syncNow();
		if (!synced) {
			showFeedback('error', 'Sync failed', 'Could not refresh from backend.');
			return;
		}
		showFeedback('success', 'Refreshed', 'Latest data loaded from backend.');
	}

	async function handleManualSync(): Promise<void> {
		if ($syncStatus.state === 'syncing') return;
		if (!isOnline) {
			showFeedback('error', 'Offline', 'You are offline. Changes will sync when the connection returns.');
			return;
		}
		if (!authSession) {
			showFeedback('error', 'Not signed in', 'Login to sync with the backend.');
			return;
		}

		const synced = await finance.syncNow();
		if (!synced) {
			showFeedback('error', 'Sync failed', $syncStatus.message || 'Could not sync with the backend.');
			return;
		}

		showFeedback('success', 'Synced', 'Your data is up to date.');
	}

	async function submitMovement(event: SubmitEvent, view: 'mobile' | 'desktop') {
		if (consumeSelectClickThroughGuard()) return;
		const form = event.currentTarget as HTMLFormElement;
		const formData = new FormData(form);

		try {
			await finance.addEntry(formData);
			form.reset();
			selectedDate = parseDate(todayInputValue());
			selectedEntryType = 'expense';
			selectedCurrency = 'SGD';
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

	function openSettingsSubpage(next: SettingsSubpage): void {
		settingsSubpage = next;
	}

	function openAccountEditor(accountId: string): void {
		selectedSettingsAccountId = selectedSettingsAccountId === accountId ? '' : accountId;
		desktopScreen = 'accounts';
		desktopAddAccountWizardOpen = false;
		settingsSubpage = 'accountEdit';
	}

	function openCategoryEditor(categoryId: string): void {
		selectedSettingsCategoryId = selectedSettingsCategoryId === categoryId ? '' : categoryId;
		desktopScreen = 'accounts';
		desktopAddCategoryWizardOpen = false;
		settingsSubpage = 'categoryEdit';
	}

	async function submitAccountUpdate(event: SubmitEvent): Promise<void> {
		if (!selectedSettingsAccountId) return;
		try {
			await submitAndSync((formData) => finance.updateAccount(selectedSettingsAccountId, formData), event);
			showFeedback('success', 'Account updated', 'Account details were saved.');
			settingsSubpage = 'accounts';
		} catch (error) {
			showFeedback('error', 'Failed', error instanceof Error ? error.message : 'Unable to update account.');
		}
	}

	async function submitCategoryUpdate(event: SubmitEvent): Promise<void> {
		if (!selectedSettingsCategoryId) return;
		try {
			await submitAndSync((formData) => finance.updateCategory(selectedSettingsCategoryId, formData), event);
			showFeedback('success', 'Category updated', 'Category details were saved.');
			settingsSubpage = 'categories';
		} catch (error) {
			showFeedback('error', 'Failed', error instanceof Error ? error.message : 'Unable to update category.');
		}
	}

	function categoryName(id: string): string {
		return categories.find((category) => category.id === id)?.name ?? 'Uncategorized';
	}

	function accountName(id: string): string {
		return allAccounts.find((account) => account.id === id)?.name ?? 'Account';
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
		const formattedAmount = normalizeMerchantText(transactionAmount(entry));
		const formattedBaseAmount = normalizeMerchantText(transactionBaseAmount(entry));
		const fields = [
			normalizeMerchantText(entry.merchant),
			normalizeMerchantText(entry.note ?? ''),
			normalizeMerchantText(entry.occurredOn),
			normalizeMerchantText(entry.type),
			normalizeMerchantText(entry.currency),
			normalizeMerchantText(accountName(entry.accountId)),
			normalizeMerchantText(categoryName(entry.categoryId)),
			normalizedAmount,
			formattedAmount,
			formattedBaseAmount
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

	function formatTransactionMonthLabel(monthKey: string): string {
		if (!/^\d{4}-\d{2}$/.test(monthKey)) return monthKey;
		const [year, month] = monthKey.split('-').map(Number);
		if (month < 1 || month > 12) return monthKey;
		return new Intl.DateTimeFormat(undefined, { month: 'long', year: 'numeric' }).format(
			new Date(year, month - 1, 1)
		);
	}

	function buildTransactionMonthOptions(allEntries: LedgerEntry[]): SelectOption[] {
		const monthKeys = new Set<string>([todayInputValue().slice(0, 7)]);
		for (const entry of allEntries) {
			const monthKey = entry.occurredOn.slice(0, 7);
			if (/^\d{4}-\d{2}$/.test(monthKey)) monthKeys.add(monthKey);
		}
		return [...monthKeys]
			.sort((a, b) => b.localeCompare(a))
			.map((value) => ({ value, label: formatTransactionMonthLabel(value) }));
	}

	function buildTransactionYearOptions(allEntries: LedgerEntry[]): SelectOption[] {
		const years = new Set<string>([todayInputValue().slice(0, 4)]);
		for (const entry of allEntries) {
			const year = entry.occurredOn.slice(0, 4);
			if (/^\d{4}$/.test(year)) years.add(year);
		}
		return [...years]
			.sort((a, b) => b.localeCompare(a))
			.map((value) => ({ value, label: value }));
	}

	function filterTransactionsByPeriod(
		allEntries: LedgerEntry[],
		mode: TransactionPeriodMode,
		month: string,
		year: string
	): LedgerEntry[] {
		const period = mode === 'month' ? month : year;
		return allEntries.filter((entry) => entry.occurredOn.startsWith(period));
	}

	function filterTransactionsByView(allEntries: LedgerEntry[], view: TransactionViewFilter): LedgerEntry[] {
		if (view === 'all') return allEntries;
		if (view === 'income' || view === 'expense') {
			return allEntries.filter((entry) => entry.type === view);
		}

		return allEntries.filter((entry) => {
			const category = categoryById.get(entry.categoryId);
			const scope = normalizeCategoryScope(category?.scope);
			if (view === 'household') return scope === 'household';
			if (scope !== 'user') return false;
			if (!category?.ownerUserId) return true;
			return category.ownerUserId === state?.settings.deviceUserId;
		});
	}

	function openDesktopTransactions(view: TransactionViewFilter = 'all'): void {
		transactionPeriodMode = 'month';
		selectedTransactionMonth = currentMonthKey;
		transactionViewFilter = view;
		transactionSearchQuery = '';
		desktopSearchOpen = false;
		desktopScreen = 'transactions';
	}

	function openTransactionSearch(): void {
		transactionSearchOrigin = activeScreen;
		remoteSearchError = '';
		remoteSearchResults = null;
		activeScreen = 'search';
	}

	function openMobileTransactions(): void {
		mobileTransactionCategoryId = '';
		mobileTransactionPeriodKey = currentMonthKey;
		mobileTransactionGrain = 'month';
		mobileTransactionViewFilter = 'all';
		activeScreen = 'transactions';
	}

	function openHomeCategoryTransactions(categoryId: string): void {
		mobileTransactionCategoryId = categoryId;
		mobileTransactionPeriodKey = currentSummary?.periodKey ?? periodKey(todayInputValue(), grain);
		mobileTransactionGrain = grain;
		mobileTransactionViewFilter = 'all';
		activeScreen = 'transactions';
	}

	function openReviewHistory(): void {
		mobileTransactionCategoryId = '';
		mobileTransactionPeriodKey = selectedReviewPeriodKey || currentMonthKey;
		mobileTransactionGrain = grain;
		mobileTransactionViewFilter = 'all';
		activeScreen = 'transactions';
	}

	function clearMobileTransactionFilter(): void {
		mobileTransactionCategoryId = '';
		mobileTransactionPeriodKey = currentMonthKey;
		mobileTransactionGrain = 'month';
	}

	async function openDesktopTransactionSearch(): Promise<void> {
		desktopScreen = 'transactions';
		transactionViewFilter = 'all';
		desktopSearchOpen = true;
		await tick();
		desktopSearchInput?.focus();
	}

	function closeDesktopTransactionSearch(): void {
		desktopSearchOpen = false;
		transactionSearchQuery = '';
	}

	function handleDesktopSearchKeydown(event: KeyboardEvent): void {
		if (event.key !== 'Escape') return;
		event.preventDefault();
		closeDesktopTransactionSearch();
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
		restoreMobileTransactionsScroll = activeScreen === 'transactions';
		if (restoreMobileTransactionsScroll) {
			mobileTransactionsScrollTop = transactionsScreenEl?.scrollTop ?? 0;
		}
		selectedTransactionId = entryId;
		selectedTransactionFallback = fallback ?? null;
		transactionEditMode = false;
		transactionDeleteConfirmOpen = false;
		activeScreen = 'transactionDetail';
	}

	async function closeMobileTransactionDetail(): Promise<void> {
		const shouldRestoreScroll = restoreMobileTransactionsScroll;
		const scrollTop = mobileTransactionsScrollTop;
		restoreMobileTransactionsScroll = false;
		transactionEditMode = false;
		transactionDeleteConfirmOpen = false;
		activeScreen = 'transactions';
		if (!shouldRestoreScroll) return;
		await tick();
		if (transactionsScreenEl) transactionsScreenEl.scrollTop = scrollTop;
	}

	function openDesktopTransactionDetail(
		entryId: string,
		origin: 'dashboard' | 'transactions',
		fallback?: LedgerEntry
	): void {
		selectedTransactionId = entryId;
		selectedTransactionFallback = fallback ?? null;
		transactionEditMode = false;
		transactionDeleteConfirmOpen = false;
		desktopTransactionDetailOrigin = origin;
		desktopScreen = 'transactionDetail';
	}

	function closeDesktopTransactionDetail(): void {
		transactionEditMode = false;
		transactionDeleteConfirmOpen = false;
		desktopScreen = desktopTransactionDetailOrigin;
	}

	function openTransactionEditor(): void {
		if (!selectedTransaction) return;
		transactionDeleteConfirmOpen = false;
		transactionEditMode = true;
		transactionEditType = selectedTransaction.type;
		transactionEditAccountId = selectedTransaction.accountId;
		transactionEditCategoryId = selectedTransaction.categoryId;
		transactionEditCurrency = normalizeCurrencyCode(selectedTransaction.currency);
		transactionEditAmount = amountFromCents(selectedTransaction.amount);
		transactionEditDate = selectedTransaction.occurredOn;
		transactionEditDateValue = parseDate(selectedTransaction.occurredOn);
		transactionEditMerchant = selectedTransaction.merchant;
		transactionEditNote = selectedTransaction.note;
	}

	function closeTransactionEditor(): void {
		transactionEditMode = false;
	}

	async function deleteSelectedTransaction(): Promise<void> {
		if (!selectedTransaction || transactionDeleting) return;
		const transactionId = selectedTransaction.id;
		const groupId = state?.settings.activeGroupId ?? '';
		let deletedLocally = false;
		transactionDeleting = true;

		try {
			await finance.deleteEntry(transactionId);
			deletedLocally = true;
			selectedTransactionId = '';
			selectedTransactionFallback = null;
			transactionDeleteConfirmOpen = false;
			if (activeScreen === 'transactionDetail') await closeMobileTransactionDetail();
			if (desktopScreen === 'transactionDetail') closeDesktopTransactionDetail();

			if (navigator.onLine && authSession && groupId) {
				const serverEntry = await deleteTransactionRemote({
					groupId,
					transactionId
				});
				await finance.acceptServerEntry(serverEntry);
			}

			let synced = false;
			if (navigator.onLine) {
				synced = await finance.syncNow();
			}

			if (!navigator.onLine) {
				showFeedback('success', 'Deleted', 'Transaction deleted locally. The change will sync when online.');
				return;
			}
			if (!synced) {
				showFeedback('error', 'Deleted locally', 'Transaction was removed locally, but backend sync failed.');
				return;
			}
			showFeedback('success', 'Deleted', 'Transaction deleted successfully.');
		} catch (error) {
			showFeedback(
				'error',
				deletedLocally ? 'Deleted locally' : 'Delete failed',
				deletedLocally
					? 'Transaction was removed locally, but backend sync failed. It will retry on the next sync.'
					: error instanceof Error
						? error.message
						: 'Unable to delete transaction.'
			);
		} finally {
			transactionDeleting = false;
		}
	}

	async function submitTransactionUpdate(event: SubmitEvent): Promise<void> {
		if (consumeSelectClickThroughGuard()) return;
		if (!selectedTransaction) return;
		const form = event.currentTarget as HTMLFormElement;
		const formData = new FormData(form);
		const accountId = transactionEditAccountId.trim();
		const categoryId = transactionEditCategoryId.trim();
		const type = transactionEditType;
		const currencyCode = normalizeCurrencyCode(transactionEditCurrency);

		// AppSelect is a custom control. Serialize its bound values explicitly so
		// local persistence and the remote update always receive the new selection.
		formData.set('accountId', accountId);
		formData.set('categoryId', categoryId);
		formData.set('type', type);
		formData.set('currency', currencyCode);
		try {
			await finance.updateEntry(selectedTransaction.id, formData);
			transactionEditMode = false;
			if (navigator.onLine && authSession && state) {
				const serverEntry = await updateTransactionRemote({
					groupId: state.settings.activeGroupId,
					transactionId: selectedTransaction.id,
					accountId,
					categoryId,
					type,
					amount: cents(formData.get('amount')),
					currency: currencyCode,
					occurredOn: `${formData.get('occurredOn') ?? todayInputValue()}`.trim(),
					merchant: `${formData.get('merchant') ?? ''}`.trim(),
					note: `${formData.get('note') ?? ''}`.trim()
				});
				await finance.acceptServerEntry(serverEntry);
			}
			let synced = false;
			if (navigator.onLine) {
				synced = await finance.syncNow();
			}
			if (!navigator.onLine) {
				showFeedback('success', 'Saved', 'Transaction updated locally. It will sync when online.');
				return;
			}
			if (!synced) {
				showFeedback('error', 'Saved locally', 'Transaction updated, but backend sync failed.');
				return;
			}
			showFeedback('success', 'Updated', 'Transaction updated successfully.');
		} catch (error) {
			showFeedback('error', 'Failed', error instanceof Error ? error.message : 'Unable to update transaction.');
		}
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

	function currency(amountInCents: number, currencyCode = baseCurrency): string {
		return formatCurrency(amountInCents, currencyCode);
	}

	function transactionAmount(entry: LedgerEntry): string {
		return currency(entry.amount, normalizeCurrencyCode(entry.currency));
	}

	function transactionBaseAmount(entry: LedgerEntry): string {
		const amount = entryAmountInBaseCurrency(entry, baseCurrency);
		return amount > 0 || entry.amount === 0 ? currency(amount, baseCurrency) : 'Pending FX conversion';
	}

	function isForeignTransaction(entry: LedgerEntry): boolean {
		return normalizeCurrencyCode(entry.currency) !== baseCurrency;
	}

	function fxRateLabel(entry: LedgerEntry): string {
		const markup = Number(entry.metadata?.fxMarkupPercent);
		return Number.isFinite(markup) && markup > 0 ? `FX rate (incl. ${markup}% buffer)` : 'FX rate';
	}

	function transactionBaseListAmount(entry: LedgerEntry): string {
		const amount = entryAmountInBaseCurrency(entry, baseCurrency);
		if (amount <= 0 && entry.amount !== 0) return 'FX pending';
		return `≈ ${entry.type === 'expense' ? '-' : '+'}${currency(amount, baseCurrency)}`;
	}

	function amountFromCents(value: number): string {
		return (Math.max(0, value) / 100).toFixed(2);
	}

	function displayColor(color: string | undefined, index: number): string {
		const normalized = color?.trim().toLowerCase();
		if (normalized && legacyColorMap[normalized]) return legacyColorMap[normalized];
		if (normalized && palette.includes(normalized)) return normalized;
		return palette[index % palette.length];
	}

	function accountMonthlyBalance(accountId: string): number {
		return entries
			.filter((entry) => entry.accountId === accountId && entry.occurredOn.startsWith(currentMonthKey))
			.reduce(
				(sum, entry) =>
					sum +
					(entry.type === 'income'
						? entryAmountInBaseCurrency(entry, baseCurrency)
						: -entryAmountInBaseCurrency(entry, baseCurrency)),
				0
			);
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

	function buildSpendingCategoryLegend(periods: PeriodSummary[]): StatItem[] {
		const categoryTotals = new Map<string, StatItem>();
		for (const period of periods) {
			for (const category of period.categories) {
				if (category.spent <= 0) continue;
				const existing = categoryTotals.get(category.categoryId);
				if (existing) {
					existing.amount += category.spent;
				} else {
					categoryTotals.set(category.categoryId, {
						name: category.categoryName,
						color: category.categoryColor,
						amount: category.spent
					});
				}
			}
		}
		return [...categoryTotals.values()].sort((a, b) => b.amount - a.amount);
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
				color: category.color,
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

	function normalizeCategoryScope(value: string | null | undefined): CategoryScope {
		return value?.toLowerCase().trim() === 'user' ? 'user' : 'household';
	}

	function isCategoryVisibleToUser(
		category: { scope?: string | null; ownerUserId?: string | null },
		userId: string
	): boolean {
		const scope = normalizeCategoryScope(category.scope);
		if (scope === 'household') return true;
		if (!userId) return false;
		if (!category.ownerUserId) return true;
		return category.ownerUserId === userId;
	}

	function categoryTypeLabel(categoryType: CategoryType): string {
		return categoryType === 'income' ? 'Income' : 'Expense';
	}

	function categoryScopeLabel(categoryScope: CategoryScope): string {
		return categoryScope === 'user' ? 'User' : 'Household';
	}

function getEntryCategoryOptions(
	allCategories: typeof categories,
	entryType: 'expense' | 'income'
): typeof categories {
	return allCategories.filter((category) => normalizeCategoryType(category.type, category.name) === entryType);
}

	function countTransactionUsage(
		allEntries: LedgerEntry[],
		field: 'accountId' | 'categoryId'
	): Map<string, number> {
		const counts = new Map<string, number>();
		for (const entry of allEntries) {
			const id = entry[field];
			counts.set(id, (counts.get(id) ?? 0) + 1);
		}
		return counts;
	}

	function sortByTransactionUsage<T extends { id: string }>(items: T[], counts: Map<string, number>): T[] {
		return items
			.map((item, index) => ({ item, index, count: counts.get(item.id) ?? 0 }))
			.sort((a, b) => b.count - a.count || a.index - b.index)
			.map(({ item }) => item);
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

	async function centerSelectedReviewPeriod(rail: HTMLElement, periodKeyValue: string): Promise<void> {
		await tick();
		const selected = [...rail.querySelectorAll<HTMLButtonElement>('[data-period-key]')].find(
			(button) => button.dataset.periodKey === periodKeyValue
		);
		if (!selected) return;
		const targetLeft = selected.offsetLeft - (rail.clientWidth - selected.offsetWidth) / 2;
		rail.scrollTo({ left: Math.max(0, targetLeft), behavior: 'smooth' });
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
				const amountDifference =
					entryAmountInBaseCurrency(b, baseCurrency) - entryAmountInBaseCurrency(a, baseCurrency);
				if (amountDifference !== 0) return amountDifference;
				return `${b.occurredOn}${b.createdAt}`.localeCompare(`${a.occurredOn}${a.createdAt}`);
			})
			.slice(0, 5);
	}

	function getDesktopHeading(screen: DesktopScreen, groupName?: string): { title: string; subtitle: string } {
		if (screen === 'statements') {
			return {
				title: 'Statement imports',
				subtitle: `Private on-device parsing and durable review for ${groupName ?? 'your household'}.`
			};
		}
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
		if (screen === 'transactionDetail') {
			return {
				title: 'Transaction detail',
				subtitle: `View and edit this ledger entry for ${groupName ?? 'your household'}.`
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
		const inviteCode = String(data.get('inviteCode') ?? '').trim();

		authLoading = true;
		authError = '';
		try {
			authSession =
				authMode === 'register'
					? await register(email, password, displayName, inviteCode)
					: await login(email, password);
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
					<input name="inviteCode" placeholder="Invite code (optional)" />
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
		<section
			bind:this={homeScreenEl}
			aria-label="Home dashboard"
			class="screen home-screen pull-refresh-screen"
			style={`--pull-offset:${pullRefreshScreen === 'home' ? `${pullDistance}px` : '0px'}`}
			on:touchstart|passive={(event) => handlePullStart('home', event)}
			on:touchmove|passive={(event) => handlePullMove('home', event)}
			on:touchend={() => handlePullEnd('home')}
			on:touchcancel={() => handlePullEnd('home')}
		>
			<div class="pull-refresh-indicator" class:visible={pullRefreshScreen === 'home' && (pullDistance > 0 || pullRefreshing)}>
				<RefreshCw size={14} class={pullRefreshing ? 'spinning' : ''} />
				<span>{pullRefreshing ? 'Refreshing...' : 'Pull to refresh'}</span>
			</div>
			<header class="hero-header">
				<div>
					<p>Hello,</p>
					<h1>{activeGroup?.name ?? 'Household'}!</h1>
				</div>
				<div class="round-actions">
					<button title="Search transactions" type="button" on:click={openTransactionSearch}><Search size={22} /></button>
				</div>
			</header>

			<div class="home-period-toggle" role="group" aria-label="Home balance period">
				<button
					type="button"
					class:active={homeBalancePeriod === 'today'}
					aria-pressed={homeBalancePeriod === 'today'}
					on:click={() => (homeBalancePeriod = 'today')}
				>
					Today
				</button>
				<button
					type="button"
					class:active={homeBalancePeriod === 'month'}
					aria-pressed={homeBalancePeriod === 'month'}
					on:click={() => (homeBalancePeriod = 'month')}
				>
					This month
				</button>
			</div>

			<button class="balance-card" type="button" on:click={() => (activeScreen = 'add')}>
				<span>Household balance</span>
				<strong>{currency(homeHouseholdBalance)}</strong>
				<small>{homeBalancePeriodLabel} · {homeHouseholdEntries.length} transaction{homeHouseholdEntries.length === 1 ? '' : 's'}</small>
			</button>

			<article class="balance-card personal-balance-card">
				<span>Personal balance</span>
				<strong>{currency(homePersonalBalance)}</strong>
				<small>{homeBalancePeriodLabel} · {homePersonalEntries.length} transaction{homePersonalEntries.length === 1 ? '' : 's'}</small>
			</article>

			<div class="home-stat-grid">
				<article>
					<span>Income</span>
					<strong>{currency(homeIncome)}</strong>
				</article>
				<article>
					<span>Spending</span>
					<strong>{currency(homeSpent)}</strong>
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
					<strong>{currency(homePersonalIncome)}</strong>
				</article>
				<article>
					<span>Personal expense</span>
					<strong>{currency(homePersonalSpent)}</strong>
				</article>
			</div>

			<section class="section-heading">
				<h2>Top categories</h2>
				<button type="button" on:click={() => (activeScreen = 'review')}>Review</button>
			</section>
			<div class="payment-cards">
				{#each homeTiles as category, index}
					<button
						type="button"
						class:accent-card={index === 0}
						class="mini-card"
						on:click={() => openHomeCategoryTransactions(category.id)}
					>
						<span style={`--swatch:${category.color}`}>
							{entryIcon(category.name)}
						</span>
						<h3>{category.name}</h3>
						<p>{currency(category.amount)}</p>
						<small>{grain} view</small>
					</button>
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
									<small>{categoryScopeLabel(category.scope)} · {categoryTypeLabel(category.type)} · Target {currency(category.target)}</small>
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
				<button type="button" on:click={openMobileTransactions}>See all</button>
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
							{entry.type === 'expense' ? '-' : '+'}{transactionAmount(entry)}
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

			<div class="month-rail" bind:this={reviewPeriodRailEl} aria-label={`Select ${grain}`}>
				{#each reviewPeriodSummaries as summary}
					<button
						type="button"
						class:active={summary.periodKey === selectedReviewPeriodKey}
						aria-current={summary.periodKey === selectedReviewPeriodKey ? 'date' : undefined}
						data-period-key={summary.periodKey}
						on:click={() => (selectedReviewPeriodKey = summary.periodKey)}
					>
						{shortPeriodLabel(summary.periodKey, grain)}
					</button>
				{/each}
			</div>

			<section class="stat-ring-card">
				<div class="ring-wrap">
					<svg class="ring-svg" viewBox="0 0 220 220" aria-label="Category expense ring">
						<circle class="ring-track" cx="110" cy="110" r="90" pathLength="565" />
						{#each reviewStatItems as item, index}
							<circle class="ring-segment" cx="110" cy="110" r="90" pathLength="565" style={ringSegmentStyle(reviewStatItems, index)} />
						{/each}
					</svg>
					<div>
						<strong>{currency(reviewTotal)}</strong>
						<span>{selectedReviewSummary ? reviewTotalLabel : 'No activity'}</span>
					</div>
				</div>
			</section>

			<div class="stat-legend">
				{#each reviewStatItems as item}
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
					<span>{selectedReviewSummary ? readablePeriod(selectedReviewSummary.periodKey, grain) : 'No period'}</span>
				</div>
				<div class="transaction-list">
					{#each reviewTopTransactions as entry}
						<button
							class="transaction-row review-transaction-row"
							type="button"
							aria-label={`Open transaction ${entry.merchant} ${transactionAmount(entry)}`}
							on:click={() => openTransactionDetail(entry.id)}
						>
							<div class="avatar">{entryIcon(entry.merchant)}</div>
							<div>
								<h3>{entry.merchant}</h3>
								<p>{formatDate(entry.occurredOn)} · {accountName(entry.accountId)}</p>
							</div>
							<strong class:negative={entry.type === 'expense'}>
								{entry.type === 'expense' ? '-' : '+'}{transactionAmount(entry)}
							</strong>
						</button>
					{:else}
						<p class="empty-card">No {reviewEntryType} transactions in this period.</p>
					{/each}
				</div>
			</section>

			<button class="history-pill" type="button" on:click={openReviewHistory}>History</button>
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
				<div class="field-grid">
					<label>
						Amount
						<input name="amount" type="text" inputmode="decimal" placeholder="0.00" on:input={formatAmountInput} required />
					</label>
					<label>
						Currency
						<AppSelect ariaLabel="Transaction currency" bind:value={selectedCurrency} name="currency" options={currencyOptions} required />
					</label>
				</div>
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
							disabled={accounts.length === 0}
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
									<ChevronDown size={18} />
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
			<section
				bind:this={transactionsScreenEl}
				aria-label="Transactions"
				class="screen pull-refresh-screen"
				style={`--pull-offset:${pullRefreshScreen === 'transactions' ? `${pullDistance}px` : '0px'}`}
				on:touchstart|passive={(event) => handlePullStart('transactions', event)}
				on:touchmove|passive={(event) => handlePullMove('transactions', event)}
				on:touchend={() => handlePullEnd('transactions')}
				on:touchcancel={() => handlePullEnd('transactions')}
			>
				<div class="pull-refresh-indicator" class:visible={pullRefreshScreen === 'transactions' && (pullDistance > 0 || pullRefreshing)}>
					<RefreshCw size={14} class={pullRefreshing ? 'spinning' : ''} />
					<span>{pullRefreshing ? 'Refreshing...' : 'Pull to refresh'}</span>
				</div>
				<header class="screen-header">
					<span class="header-spacer"></span>
					<h1>Transactions</h1>
					<button class="plain-icon-button" title="Search transactions" type="button" on:click={openTransactionSearch}>
						<Search size={18} />
					</button>
			</header>

			<section class="transaction-table-card">
				<div class="mobile-transaction-controls">
					{#if mobileTransactionFilterLabel}
						<div class="mobile-transaction-filter">
							{#if mobileTransactionGrain === 'month'}
								<AppSelect
									ariaLabel="Transaction month"
									bind:value={mobileTransactionPeriodKey}
									options={transactionMonthOptions}
									triggerClass="mobile-transaction-month-trigger"
								/>
							{:else}
								<span>{mobileTransactionFilterLabel}</span>
							{/if}
							{#if mobileTransactionCategoryId}
								<button type="button" on:click={clearMobileTransactionFilter} aria-label="Clear category filter">×</button>
							{/if}
						</div>
					{/if}
					<AppSelect
						ariaLabel="Filter transactions"
						bind:value={mobileTransactionViewFilter}
						options={transactionViewFilterOptions}
						triggerClass="mobile-transaction-view-trigger"
					/>
				</div>
				<div class="transaction-table-head">
					<span>Date</span>
					<span>Merchant</span>
					<span>Category</span>
					<span>Amount</span>
				</div>
				<div class="transaction-table">
					{#each mobileTransactions as entry}
						<button
							type="button"
							aria-label={`Open transaction ${entry.merchant} ${transactionAmount(entry)}`}
							on:click={() => openTransactionDetail(entry.id)}
						>
							<time>{formatDate(entry.occurredOn)}</time>
							<div>
								<strong>{entry.merchant}</strong>
								<small>{accountName(entry.accountId)}</small>
							</div>
							<span>{categoryName(entry.categoryId)}</span>
							<div class="transaction-amount-cell">
								<b class:negative={entry.type === 'expense'}>
									{entry.type === 'expense' ? '-' : '+'}{transactionAmount(entry)}
								</b>
								{#if isForeignTransaction(entry)}
									<small class="transaction-base-amount">{transactionBaseListAmount(entry)}</small>
								{/if}
							</div>
						</button>
					{:else}
						<p class="empty-card">
							{mobileTransactionCategoryId
								? `No ${mobileTransactionEmptyLabel} for ${mobileTransactionFilterLabel}.`
								: `No ${mobileTransactionEmptyLabel} in ${mobileTransactionPeriodLabel}.`}
						</p>
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
							aria-label={`Open transaction ${entry.merchant} ${transactionAmount(entry)}`}
							on:click={() => openTransactionDetail(entry.id, entry)}
						>
							<div>
								<strong>{entry.merchant}</strong>
								<small>{formatDate(entry.occurredOn)} · {categoryName(entry.categoryId)} · {accountName(entry.accountId)}</small>
							</div>
							<div class="transaction-amount-cell">
								<b class:negative={entry.type === 'expense'}>
									{entry.type === 'expense' ? '-' : '+'}{transactionAmount(entry)}
								</b>
								{#if isForeignTransaction(entry)}
									<small class="transaction-base-amount">{transactionBaseListAmount(entry)}</small>
								{/if}
							</div>
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
					<button class="plain-icon-button" title="Close" type="button" on:click={closeMobileTransactionDetail}>×</button>
				</header>
			{#if selectedTransaction}
				{#if transactionEditMode}
					<form class="form-card transaction-edit-form" on:submit|preventDefault={submitTransactionUpdate}>
						<div class="field-grid">
							<label>
								Type
								<AppSelect ariaLabel="Entry type" bind:value={transactionEditType} name="type" options={entryTypeOptions} required />
							</label>
							<label>
								Amount
								<input bind:value={transactionEditAmount} name="amount" type="text" inputmode="decimal" on:input={formatAmountInput} required />
							</label>
						</div>
						<label>
							Currency
							<AppSelect
								ariaLabel="Transaction currency"
								bind:value={transactionEditCurrency}
								name="currency"
								options={currencyOptions}
								required
							/>
						</label>
						<label>
							Merchant
							<input bind:value={transactionEditMerchant} name="merchant" required />
						</label>
						<div class="field-grid">
							<label>
								Account
								<AppSelect
									ariaLabel="Account"
									bind:value={transactionEditAccountId}
									disabled={accounts.length === 0}
									name="accountId"
									options={accountSelectOptions}
									required
								/>
							</label>
							<label>
								Category
								<AppSelect
									ariaLabel="Category"
									bind:value={transactionEditCategoryId}
									disabled={transactionEditCategoryOptions.length === 0}
									name="categoryId"
									options={transactionEditCategoryOptions}
									required
								/>
							</label>
						</div>
						<div class="field-grid transaction-date-note-grid">
							<label>
								Date
								<DatePicker.Root bind:value={transactionEditDateValue} weekdayFormat="short" fixedWeeks={true}>
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
											<ChevronDown size={18} />
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
								<input bind:value={transactionEditNote} name="note" placeholder="Optional" />
							</label>
						</div>
						<div class="button-row">
							<button type="submit">Save changes</button>
							<button class="ghost" type="button" on:click={closeTransactionEditor}>Cancel</button>
						</div>
					</form>
				{:else}
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
								{selectedTransaction.type === 'expense' ? '-' : '+'}{transactionAmount(selectedTransaction)}
							</strong>
						</div>
						<div class="transaction-detail-row">
							<span>Currency</span>
							<strong>{normalizeCurrencyCode(selectedTransaction.currency)}</strong>
						</div>
						{#if normalizeCurrencyCode(selectedTransaction.currency) !== baseCurrency}
							<div class="transaction-detail-row">
								<span>In {baseCurrency}</span>
								<strong>{transactionBaseAmount(selectedTransaction)}</strong>
							</div>
							<div class="transaction-detail-row">
								<span>{fxRateLabel(selectedTransaction)}</span>
								<strong>{selectedTransaction.fxRate > 0 ? selectedTransaction.fxRate.toFixed(6) : 'Pending'}</strong>
							</div>
							<div class="transaction-detail-row">
								<span>FX date</span>
								<strong>{selectedTransaction.fxRateDate ? formatDate(selectedTransaction.fxRateDate) : 'Pending'}</strong>
							</div>
						{/if}
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
						<div class="button-row transaction-detail-actions">
							{#if transactionDeleteConfirmOpen}
								<p class="transaction-delete-confirmation">Delete this transaction? This cannot be undone.</p>
								<button class="danger" type="button" disabled={transactionDeleting} on:click={deleteSelectedTransaction}>
									{transactionDeleting ? 'Deleting...' : 'Confirm delete'}
								</button>
								<button class="ghost" type="button" disabled={transactionDeleting} on:click={() => (transactionDeleteConfirmOpen = false)}>
									Cancel
								</button>
							{:else}
								<button type="button" on:click={openTransactionEditor}>Edit transaction</button>
								<button class="danger" type="button" on:click={() => (transactionDeleteConfirmOpen = true)}>Delete transaction</button>
							{/if}
						</div>
					</section>
				{/if}
			{:else}
				<p class="empty-card">Transaction not found.</p>
			{/if}
		</section>
	{:else}
		<section class="screen settings-screen" class:statement-screen={settingsSubpage === 'statements'}>
			{#if settingsSubpage !== 'statements'}
			<header class="screen-header">
				<span class="header-spacer"></span>
				<h1>Settings</h1>
				<span class:offline={!isOnline} class="connection">
					{#if isOnline}<Wifi size={16} />{:else}<WifiOff size={16} />{/if}
				</span>
			</header>
			{/if}

			{#if settingsSubpage === 'overview'}
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

				<section class="form-card settings-shortcuts">
					<h2>Manage data</h2>
					<p class="muted">Use focused pages for accounts, categories, and adjustments.</p>
					<div class="button-row">
						<button type="button" on:click={() => openSettingsSubpage('accounts')}>Modify accounts</button>
						<button type="button" on:click={() => openSettingsSubpage('categories')}>Modify categories</button>
					</div>
					<button class="ghost" type="button" on:click={() => openSettingsSubpage('adjustments')}>Manual category add/minus</button>
					<button class="ghost" type="button" on:click={() => openSettingsSubpage('statements')}>Statement imports</button>
				</section>

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
						<button
							type="button"
							disabled={$syncStatus.state === 'syncing'}
							aria-busy={$syncStatus.state === 'syncing'}
							on:click={handleManualSync}
						>
							<RefreshCw size={16} class={$syncStatus.state === 'syncing' ? 'spinning' : ''} />
							{$syncStatus.state === 'syncing' ? 'Syncing...' : 'Sync now'}
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
			{:else if settingsSubpage === 'statements'}
				<section class="settings-list">
					<button class="statement-settings-back" type="button" on:click={() => openSettingsSubpage('overview')}><ArrowLeft size={17} /> Settings</button>
					{#if activeGroup}
						<StatementIngestion groupId={activeGroup.id} {accounts} {categories} entries={entries} onConfirmed={() => finance.syncNow()} />
					{/if}
				</section>
			{:else if settingsSubpage === 'accounts'}
				<section class="settings-list clickable-settings-list">
					<div class="settings-subpage-heading">
						<h2>Modify accounts</h2>
						<button class="ghost" type="button" on:click={() => openSettingsSubpage('overview')}>Back to settings</button>
					</div>
					{#each allAccounts as account}
						<button
							class="settings-row-button"
							class:inactive-account={!isActive(account)}
							type="button"
							on:click={() => openAccountEditor(account.id)}
						>
							<span style={`--swatch:${account.color}`}></span>
							<div>
								<strong>{accountEmoji(account)} {account.name}</strong>
								<small>{account.type} · FX {account.fxMarkupPercent}% · {isActive(account) ? 'Active' : 'Inactive'}</small>
							</div>
							<b>{currency(accountMonthlyBalance(account.id))}</b>
						</button>
					{/each}
				</section>

				<form class="form-card" on:submit|preventDefault={(event) => submitAndSync(finance.addAccount, event)}>
					<h2>Add account</h2>
					<input name="name" placeholder="Account name" required />
					<div class="field-grid">
						<AppSelect ariaLabel="Account type" bind:value={accountFormType} name="type" options={accountTypeOptions} />
						<input name="openingBalance" type="text" inputmode="decimal" placeholder="Opening" on:input={formatAmountInput} />
					</div>
					<label>
						FX markup (%)
						<input name="fxMarkupPercent" type="number" min="0" max="100" step="0.1" value="3.5" inputmode="decimal" />
					</label>
					<input name="icon" placeholder="Emoji icon (e.g. 🏦)" />
					<input name="color" type="color" value="#2563eb" title="Account color" />
					<button type="submit">Add account</button>
				</form>
			{:else if settingsSubpage === 'accountEdit'}
				<section class="form-card">
					<div class="settings-subpage-heading">
						<h2>Edit account</h2>
						<button class="ghost" type="button" on:click={() => openSettingsSubpage('accounts')}>Back to accounts</button>
					</div>
					{#if selectedSettingsAccount}
						<form on:submit|preventDefault={submitAccountUpdate}>
							<input name="name" value={selectedSettingsAccount.name} placeholder="Account name" required />
							<div class="field-grid">
								<AppSelect ariaLabel="Account type" bind:value={selectedSettingsAccountType} name="type" options={accountTypeOptions} />
								<input
									name="openingBalance"
									type="text"
									inputmode="decimal"
									value={amountFromCents(selectedSettingsAccount.openingBalance)}
									placeholder="Opening"
									on:input={formatAmountInput}
								/>
							</div>
							<label>
								FX markup (%)
								<input name="fxMarkupPercent" type="number" min="0" max="100" step="0.1" value={selectedSettingsAccount.fxMarkupPercent} inputmode="decimal" />
							</label>
							<input name="icon" value={selectedSettingsAccount.icon} placeholder="Emoji icon (e.g. 🏦)" />
							<input name="color" type="color" value={selectedSettingsAccount.color} title="Account color" />
							<label class="account-status-toggle">
								<input name="inactive" type="checkbox" checked={!isActive(selectedSettingsAccount)} />
								<span>
									<strong>Inactive account</strong>
									<small>Keep its history, but prevent it from being selected for new transactions.</small>
								</span>
							</label>
							<button type="submit">Save account</button>
						</form>
					{:else}
						<p class="empty-card">Account not found.</p>
					{/if}
				</section>
			{:else if settingsSubpage === 'categories'}
				<section class="settings-list clickable-settings-list">
					<div class="settings-subpage-heading">
						<h2>Modify categories</h2>
						<button class="ghost" type="button" on:click={() => openSettingsSubpage('overview')}>Back to settings</button>
					</div>
					{#each allCategories as category}
						<button
							class="settings-row-button"
							class:inactive-category={!isActive(category)}
							type="button"
							on:click={() => openCategoryEditor(category.id)}
						>
							<span style={`--swatch:${category.color}`}></span>
							<div>
								<strong>{categoryEmoji(category)} {category.name}</strong>
								<small>
									{categoryScopeLabel(category.scope)} · {categoryTypeLabel(category.type)} · Target
									{currency(category.monthlyTarget)}{isActive(category) ? '' : ' · Inactive'}
								</small>
							</div>
							<b>{currency(currentCategoryTotals.get(category.id)?.net ?? 0)}</b>
						</button>
					{/each}
				</section>

				<form class="form-card" on:submit|preventDefault={(event) => submitAndSync(finance.addCategory, event)}>
					<h2>Add category</h2>
					<input name="name" placeholder="Category name" required />
					<div class="field-grid">
						<AppSelect ariaLabel="Category type" bind:value={categoryFormType} name="type" options={categoryTypeOptions} />
						<AppSelect ariaLabel="Category scope" bind:value={categoryFormScope} name="scope" options={categoryScopeOptions} />
					</div>
					<div class="field-grid">
						<input name="monthlyTarget" type="text" inputmode="decimal" placeholder="Monthly target" on:input={formatAmountInput} />
					</div>
					<input name="icon" placeholder="Emoji icon (e.g. 🛒)" />
					<input name="color" type="color" value="#10b981" title="Category color" />
					<button type="submit">Add category</button>
				</form>
			{:else if settingsSubpage === 'categoryEdit'}
				<section class="form-card">
					<div class="settings-subpage-heading">
						<h2>Edit category</h2>
						<button class="ghost" type="button" on:click={() => openSettingsSubpage('categories')}>Back to categories</button>
					</div>
					{#if selectedSettingsCategory}
						<form on:submit|preventDefault={submitCategoryUpdate}>
							<input name="name" value={selectedSettingsCategory.name} placeholder="Category name" required />
							<div class="field-grid">
								<AppSelect
									ariaLabel="Category type"
									bind:value={selectedSettingsCategoryType}
									name="type"
									options={categoryTypeOptions}
								/>
								<AppSelect
									ariaLabel="Category scope"
									bind:value={selectedSettingsCategoryScope}
									name="scope"
									options={categoryScopeOptions}
								/>
							</div>
							<div class="field-grid">
								<input
									name="monthlyTarget"
									type="text"
									inputmode="decimal"
									value={amountFromCents(selectedSettingsCategory.monthlyTarget)}
									placeholder="Monthly target"
									on:input={formatAmountInput}
								/>
							</div>
							<input name="icon" value={selectedSettingsCategory.icon} placeholder="Emoji icon (e.g. 🛒)" />
							<input name="color" type="color" value={selectedSettingsCategory.color} title="Category color" />
							<label class="account-status-toggle">
								<input name="inactive" type="checkbox" checked={!isActive(selectedSettingsCategory)} />
								<span>
									<strong>Inactive category</strong>
									<small>Keep its transaction history, but prevent it from being selected for new transactions.</small>
								</span>
							</label>
							<button type="submit">Save category</button>
						</form>
					{:else}
						<p class="empty-card">Category not found.</p>
					{/if}
				</section>
			{:else if settingsSubpage === 'adjustments'}
				<form class="form-card" on:submit|preventDefault={(event) => submitAndSync(finance.addAdjustment, event)}>
					<div class="settings-subpage-heading">
						<h2>Manual category add/minus</h2>
						<button class="ghost" type="button" on:click={() => openSettingsSubpage('overview')}>Back to settings</button>
					</div>
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
						<DatePicker.Root bind:value={selectedDate} weekdayFormat="short" fixedWeeks={true}>
							<div class="date-picker-field">
								<DatePicker.Input name="occurredOn" class="date-picker-input">
									{#snippet children({ segments })}
										{#each segments as segment, index (`settings-adjustment-${segment.part}-${index}`)}
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
									<ChevronDown size={18} />
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
					</div>
					<input name="note" placeholder="Adjustment note" />
					<button type="submit">Adjust category</button>
				</form>
			{/if}
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
			on:click={openMobileTransactions}
		>
			<ClipboardList size={23} />
			<span>Transactions</span>
		</button>
		<button
			class:active={activeScreen === 'settings'}
			title="Settings"
			type="button"
			on:click={() => {
				activeScreen = 'settings';
				settingsSubpage = 'overview';
			}}
		>
			<Settings size={23} />
			<span>Settings</span>
		</button>
	</nav>
</main>

{#if feedbackOpen}
	<div class="feedback-toast-wrap" role="status" aria-live="polite">
		<div class={`feedback-toast ${feedbackKind}`}>
			<div class="feedback-icon" aria-hidden="true">{feedbackKind === 'success' ? '✓' : '!'}</div>
			<div class="feedback-copy">
				<h3>{feedbackTitle}</h3>
				<p>{feedbackMessage}</p>
			</div>
		</div>
	</div>
{/if}

<main class="desktop-app" class:statement-mode={desktopScreen === 'statements'}>
	<aside class="desktop-sidebar">
		<div class="brand-mark">
			<span>F</span>
			<strong>FinSet</strong>
		</div>
		<nav aria-label="Desktop sections">
			<a href="#dashboard" class:active={desktopScreen === 'dashboard'} on:click|preventDefault={() => (desktopScreen = 'dashboard')}>
				<Home size={18} /> Dashboard
			</a>
			<a
				href="#transactions"
				class:active={desktopScreen === 'transactions' || desktopScreen === 'transactionDetail'}
				on:click|preventDefault={() => openDesktopTransactions()}
			>
				<ClipboardList size={18} /> Transactions
			</a>
			<a href="#statements" class:active={desktopScreen === 'statements'} on:click|preventDefault={() => (desktopScreen = 'statements')}>
				<FileScan size={18} /> Statements
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
			<button
				type="button"
				disabled={$syncStatus.state === 'syncing'}
				aria-busy={$syncStatus.state === 'syncing'}
				on:click={handleManualSync}
			>
				<RefreshCw size={17} class={$syncStatus.state === 'syncing' ? 'spinning' : ''} />
				{$syncStatus.state === 'syncing' ? 'Syncing...' : 'Sync'}
			</button>
			<span class:offline={!isOnline}>{isOnline ? 'Online' : 'Offline'}</span>
		</div>
	</aside>

	<section class="desktop-main" id="dashboard">
		{#if desktopScreen !== 'statements'}
		<header class="desktop-topbar">
			<div>
				<h1>{desktopHeading.title}</h1>
				<p>{desktopHeading.subtitle}</p>
			</div>
				<div class="desktop-actions">
					{#if desktopSearchOpen}
						<div class="desktop-search-control open" role="search">
							<Search size={18} aria-hidden="true" />
							<input
								bind:this={desktopSearchInput}
								bind:value={transactionSearchQuery}
								type="search"
								placeholder="Search transactions..."
								aria-label="Search transactions"
								on:keydown={handleDesktopSearchKeydown}
							/>
							<button
								class="desktop-search-close"
								title="Close search"
								aria-label="Close search"
								type="button"
								on:click={closeDesktopTransactionSearch}
							>×</button>
						</div>
					{:else}
						<button
							class="desktop-search-trigger"
							title="Search transactions"
							aria-label="Search transactions"
							aria-expanded="false"
							type="button"
							on:click={openDesktopTransactionSearch}
						>
							<Search size={20} />
						</button>
					{/if}
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
		{/if}

			{#if desktopScreen === 'dashboard'}
			<div class="desktop-filter-row">
				<div class="home-period-toggle desktop-home-period-toggle" role="group" aria-label="Home balance period">
					<button
						type="button"
						class:active={homeBalancePeriod === 'today'}
						aria-pressed={homeBalancePeriod === 'today'}
						on:click={() => (homeBalancePeriod = 'today')}
					>
						Today
					</button>
					<button
						type="button"
						class:active={homeBalancePeriod === 'month'}
						aria-pressed={homeBalancePeriod === 'month'}
						on:click={() => (homeBalancePeriod = 'month')}
					>
						This month
					</button>
				</div>
				<button type="button" on:click={() => (desktopScreen = 'settings')}><Settings size={17} /> Manage widgets</button>
			</div>

		<section class="desktop-kpis">
			<button type="button" on:click={() => openDesktopTransactions('household')}>
				<span>Household balance</span>
				<strong>{currency(homeHouseholdBalance)}</strong>
				<small>{homeBalancePeriodLabel} · {homeHouseholdEntries.length} transaction{homeHouseholdEntries.length === 1 ? '' : 's'}</small>
			</button>
			<button type="button" on:click={() => openDesktopTransactions('personal')}>
				<span>Personal balance</span>
				<strong>{currency(homePersonalBalance)}</strong>
				<small>{homeBalancePeriodLabel} · {homePersonalEntries.length} personal transaction{homePersonalEntries.length === 1 ? '' : 's'}</small>
			</button>
			<button type="button" on:click={() => openDesktopTransactions('income')}>
				<span>Income</span>
				<strong>{currency(homeIncome)}</strong>
				<small>{homeBalancePeriodLabel} · {homeIncomeEntries.length} transaction{homeIncomeEntries.length === 1 ? '' : 's'}</small>
			</button>
			<button type="button" on:click={() => openDesktopTransactions('expense')}>
				<span>Expense</span>
				<strong>{currency(homeSpent)}</strong>
				<small>{homeBalancePeriodLabel} · {homeExpenseEntries.length} transaction{homeExpenseEntries.length === 1 ? '' : 's'}</small>
			</button>
		</section>

		<div class="desktop-grid">
			<section class="desktop-card wide-card">
				<div class="desktop-card-head">
					<div>
						<h2>Spending by category</h2>
						<p>Period totals split across spending categories</p>
					</div>
					<AppSelect
						ariaLabel="Desktop report period"
						bind:value={grain}
						options={grainOptions}
						triggerClass="desktop-grain-trigger"
					/>
				</div>
				{#if desktopSpendingSeries.length}
					<div
						class="stacked-period-chart"
						style={`--period-count:${desktopSpendingSeries.length}`}
					>
						{#each desktopSpendingSeries as summary, index}
							<div class="stacked-period-column" class:latest={index === latestSpendingSeriesIndex}>
								<strong>{currency(summary.spent)}</strong>
								<div class="stacked-period-track">
									<div
										class="stacked-period-bar"
										style={`--height:${summary.spent > 0 ? Math.max(5, (summary.spent / desktopSpendingSeriesMax) * 100) : 0}%`}
										role="img"
										aria-label={`${readablePeriod(summary.periodKey, grain)} spending ${currency(summary.spent)}`}
									>
										{#each summary.categories.filter((category) => category.spent > 0) as category}
											<span
												style={`--segment:${(category.spent / Math.max(1, summary.spent)) * 100}%;--swatch:${category.categoryColor}`}
												title={`${category.categoryName}: ${currency(category.spent)}`}
											></span>
										{/each}
									</div>
								</div>
							</div>
						{/each}
					</div>
					<div class="stacked-period-axis" style={`--period-count:${desktopSpendingSeries.length}`}>
						{#each desktopSpendingSeries as summary, index}
							<span class:latest={index === latestSpendingSeriesIndex}>{shortPeriodLabel(summary.periodKey, grain)}</span>
						{/each}
					</div>
					<div class="stacked-period-legend">
						{#each desktopSpendingCategoryLegend as category}
							<span style={`--swatch:${category.color}`}>{category.name}</span>
						{/each}
					</div>
				{:else}
					<p class="muted">No spending data yet.</p>
				{/if}
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
						<strong>{currency(desktopReviewTotal)}</strong>
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
							<div class="bar-stack" style={`--swatch:${item.color}`} aria-hidden="true">
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
						<p>This month across cards, banks, wallets and cash</p>
					</div>
					<button type="button" on:click={() => (desktopAddAccountWizardOpen = !desktopAddAccountWizardOpen)}>
						<Plus size={16} />
						{desktopAddAccountWizardOpen ? 'Close' : 'Add new'}
					</button>
				</div>
				<div class="desktop-account-list">
					{#each accounts as account}
						<button class="desktop-account-row" type="button" on:click={() => openAccountEditor(account.id)}>
							<div class="entity-lead">
								<span class="entity-icon">{accountEmoji(account)}</span>
								<span style={`--swatch:${account.color}`}></span>
							</div>
							<div>
								<strong>{account.name}</strong>
								<small>{account.type} · FX {account.fxMarkupPercent}% · This month</small>
							</div>
							<b>{currency(accountMonthlyBalance(account.id))}</b>
						</button>
					{/each}
				</div>
						{#if desktopAddAccountWizardOpen}
							<form class="desktop-form-grid desktop-inline-wizard" on:submit|preventDefault={submitDesktopAccountWizard}>
								<input name="name" placeholder="Account name" required />
								<div class="field-grid">
									<AppSelect ariaLabel="Account type" bind:value={accountFormType} name="type" options={accountTypeOptions} />
									<input name="openingBalance" type="text" inputmode="decimal" placeholder="Opening" on:input={formatAmountInput} />
								</div>
								<label>
									FX markup (%)
									<input name="fxMarkupPercent" type="number" min="0" max="100" step="0.1" value="3.5" inputmode="decimal" />
								</label>
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
						<button class="desktop-account-row" type="button" on:click={() => openCategoryEditor(category.id)}>
							<div class="entity-lead">
								<span class="entity-icon">{categoryEmoji(category)}</span>
								<span style={`--swatch:${category.color}`}></span>
							</div>
							<div>
								<strong>{category.name}</strong>
								<small>{categoryScopeLabel(category.scope)} · {categoryTypeLabel(category.type)} · Target {currency(category.monthlyTarget)}</small>
							</div>
							<div class="desktop-category-row-summary">
								<b>{currency(currentMonthCategoryAmounts.get(category.id) ?? 0)}</b>
								<small>This month · Edit</small>
							</div>
						</button>
					{/each}
				</div>
						{#if desktopAddCategoryWizardOpen}
							<form class="desktop-form-grid desktop-inline-wizard" on:submit|preventDefault={submitDesktopCategoryWizard}>
								<input name="name" placeholder="Category name" required />
							<div class="field-grid">
								<AppSelect ariaLabel="Category type" bind:value={categoryFormType} name="type" options={categoryTypeOptions} />
								<AppSelect ariaLabel="Category scope" bind:value={categoryFormScope} name="scope" options={categoryScopeOptions} />
							</div>
							<div class="field-grid">
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
				<button type="button" on:click={() => openDesktopTransactions()}>Open full table</button>
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
					<button
						class="desktop-transaction-row"
						type="button"
						aria-label={`Open transaction ${entry.merchant} ${transactionAmount(entry)}`}
						on:click={() => openDesktopTransactionDetail(entry.id, 'dashboard')}
					>
						<time>{formatDate(entry.occurredOn)}</time>
						<strong>{entry.merchant}</strong>
						<span>{categoryName(entry.categoryId)}</span>
						<span>{accountName(entry.accountId)}</span>
						<div class="transaction-amount-cell">
							<b class:negative={entry.type === 'expense'}>{entry.type === 'expense' ? '-' : '+'}{transactionAmount(entry)}</b>
							{#if isForeignTransaction(entry)}
								<small class="transaction-base-amount">{transactionBaseListAmount(entry)}</small>
							{/if}
						</div>
					</button>
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
							<p>
								{selectedTransactionPeriodLabel} · {selectedTransactionViewLabel} · {desktopFilteredTransactions.length}
								{desktopFilteredTransactions.length === 1 ? 'transaction' : 'transactions'}
							</p>
						</div>
					</div>
					<div class="desktop-transaction-period-controls" aria-label="Transaction period">
						<span>Show by</span>
						<AppSelect
							ariaLabel="Group transactions by month or year"
							bind:value={transactionPeriodMode}
							options={transactionPeriodModeOptions}
							triggerClass="desktop-period-mode-trigger"
						/>
						{#if transactionPeriodMode === 'month'}
							<AppSelect
								ariaLabel="Transaction month"
								bind:value={selectedTransactionMonth}
								options={transactionMonthOptions}
								triggerClass="desktop-period-value-trigger"
							/>
						{:else}
							<AppSelect
								ariaLabel="Transaction year"
								bind:value={selectedTransactionYear}
								options={transactionYearOptions}
								triggerClass="desktop-period-value-trigger"
							/>
						{/if}
						<AppSelect
							ariaLabel="Filter transactions"
							bind:value={transactionViewFilter}
							options={transactionViewFilterOptions}
							triggerClass="desktop-transaction-filter-trigger"
						/>
					</div>
					<div class="desktop-table">
						<div class="desktop-table-head">
							<span>Date</span>
							<span>Merchant</span>
							<span>Category</span>
							<span>Account</span>
							<span>Amount</span>
						</div>
						{#each desktopFilteredTransactions as entry}
							<button
								class="desktop-transaction-row"
								type="button"
								aria-label={`Open transaction ${entry.merchant} ${transactionAmount(entry)}`}
								on:click={() => openDesktopTransactionDetail(entry.id, 'transactions')}
							>
								<time>{formatDate(entry.occurredOn)}</time>
								<strong>{entry.merchant}</strong>
								<span>{categoryName(entry.categoryId)}</span>
								<span>{accountName(entry.accountId)}</span>
								<div class="transaction-amount-cell">
									<b class:negative={entry.type === 'expense'}>{entry.type === 'expense' ? '-' : '+'}{transactionAmount(entry)}</b>
									{#if isForeignTransaction(entry)}
										<small class="transaction-base-amount">{transactionBaseListAmount(entry)}</small>
									{/if}
								</div>
							</button>
						{:else}
							<p class="muted">
								{transactionSearchQueryNormalized
									? `No matching transactions in ${selectedTransactionPeriodLabel}.`
									: `No transactions in ${selectedTransactionPeriodLabel}.`}
							</p>
						{/each}
					</div>
				</section>
			{:else if desktopScreen === 'transactionDetail'}
				<section class="desktop-card desktop-page-card">
					<div class="desktop-card-head">
						<div>
							<h2>Transaction detail</h2>
							<p>Review the ledger entry or update its details.</p>
						</div>
						<button type="button" on:click={closeDesktopTransactionDetail}>← Back to transactions</button>
					</div>
					{#if selectedTransaction}
						{#if transactionEditMode}
							<form class="desktop-form-grid transaction-edit-form" on:submit|preventDefault={submitTransactionUpdate}>
								<div class="field-grid">
									<label>
										Type
										<AppSelect ariaLabel="Entry type" bind:value={transactionEditType} name="type" options={entryTypeOptions} required />
									</label>
									<label>
										Amount
										<input
											bind:value={transactionEditAmount}
											name="amount"
											type="text"
											inputmode="decimal"
											on:input={formatAmountInput}
											required
										/>
									</label>
								</div>
								<label>
									Currency
									<AppSelect
										ariaLabel="Transaction currency"
										bind:value={transactionEditCurrency}
										name="currency"
										options={currencyOptions}
										required
									/>
								</label>
								<label>
									Merchant
									<input bind:value={transactionEditMerchant} name="merchant" required />
								</label>
								<div class="field-grid">
									<label>
										Account
										<AppSelect
											ariaLabel="Account"
											bind:value={transactionEditAccountId}
											disabled={accounts.length === 0}
											name="accountId"
											options={accountSelectOptions}
											required
										/>
									</label>
									<label>
										Category
										<AppSelect
											ariaLabel="Category"
											bind:value={transactionEditCategoryId}
											disabled={transactionEditCategoryOptions.length === 0}
											name="categoryId"
											options={transactionEditCategoryOptions}
											required
										/>
									</label>
								</div>
								<div class="field-grid transaction-date-note-grid">
									<label>
										Date
										<input bind:value={transactionEditDate} name="occurredOn" type="date" required />
									</label>
									<label>
										Note
										<input bind:value={transactionEditNote} name="note" placeholder="Optional" />
									</label>
								</div>
								<div class="button-row">
									<button type="submit">Save changes</button>
									<button class="ghost" type="button" on:click={closeTransactionEditor}>Cancel</button>
								</div>
							</form>
						{:else}
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
										{selectedTransaction.type === 'expense' ? '-' : '+'}{transactionAmount(selectedTransaction)}
									</strong>
								</div>
								<div class="transaction-detail-row">
									<span>Currency</span>
									<strong>{normalizeCurrencyCode(selectedTransaction.currency)}</strong>
								</div>
								{#if normalizeCurrencyCode(selectedTransaction.currency) !== baseCurrency}
									<div class="transaction-detail-row">
										<span>In {baseCurrency}</span>
										<strong>{transactionBaseAmount(selectedTransaction)}</strong>
									</div>
									<div class="transaction-detail-row">
										<span>{fxRateLabel(selectedTransaction)}</span>
										<strong>{selectedTransaction.fxRate > 0 ? selectedTransaction.fxRate.toFixed(6) : 'Pending'}</strong>
									</div>
									<div class="transaction-detail-row">
										<span>FX date</span>
										<strong>{selectedTransaction.fxRateDate ? formatDate(selectedTransaction.fxRateDate) : 'Pending'}</strong>
									</div>
								{/if}
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
								<div class="button-row transaction-detail-actions">
									{#if transactionDeleteConfirmOpen}
										<p class="transaction-delete-confirmation">Delete this transaction? This cannot be undone.</p>
										<button class="danger" type="button" disabled={transactionDeleting} on:click={deleteSelectedTransaction}>
											{transactionDeleting ? 'Deleting...' : 'Confirm delete'}
										</button>
										<button
											class="ghost"
											type="button"
											disabled={transactionDeleting}
											on:click={() => (transactionDeleteConfirmOpen = false)}
										>
											Cancel
										</button>
									{:else}
										<button type="button" on:click={openTransactionEditor}>Edit transaction</button>
										<button class="danger" type="button" on:click={() => (transactionDeleteConfirmOpen = true)}>
											Delete transaction
										</button>
									{/if}
								</div>
							</section>
						{/if}
					{:else}
						<p class="empty-card">Transaction not found.</p>
					{/if}
				</section>
			{:else if desktopScreen === 'accounts'}
				<div class="desktop-page-grid">
					<section class="desktop-card">
						<div class="desktop-card-head">
							<div>
								<h2>Accounts</h2>
								<p>Current-month activity by cash, bank, card, and wallet.</p>
							</div>
							<button type="button" on:click={() => (desktopAddAccountWizardOpen = !desktopAddAccountWizardOpen)}>
								<Plus size={16} />
								{desktopAddAccountWizardOpen ? 'Close' : 'Add new'}
							</button>
						</div>
						<div class="desktop-account-list">
							{#each allAccounts as account}
								<Collapsible.Root
									open={selectedSettingsAccountId === account.id}
									onOpenChange={(open) => {
										selectedSettingsAccountId = open ? account.id : '';
										if (open) desktopAddAccountWizardOpen = false;
									}}
								>
									<Collapsible.Trigger class={`desktop-account-row ${isActive(account) ? '' : 'inactive-account'}`.trim()}>
										<div class="entity-lead">
											<span class="entity-icon">{accountEmoji(account)}</span>
											<span style={`--swatch:${account.color}`}></span>
										</div>
										<div>
											<strong>{account.name}</strong>
											<small>{account.type} · FX {account.fxMarkupPercent}% · {isActive(account) ? 'Active' : 'Inactive'}</small>
										</div>
										<b class:desktop-row-action={selectedSettingsAccountId === account.id}>
											{selectedSettingsAccountId === account.id ? 'Close' : currency(accountMonthlyBalance(account.id))}
										</b>
									</Collapsible.Trigger>
									<Collapsible.Content forceMount>
										{#snippet child({ props, open })}
											{#if open}
												<form
													{...props}
													class="desktop-form-grid desktop-inline-wizard desktop-row-editor"
													on:submit|preventDefault={submitAccountUpdate}
													transition:slide={{ duration: 220 }}
												>
													<input name="name" value={account.name} placeholder="Account name" required />
													<div class="field-grid">
														<AppSelect
															ariaLabel="Account type"
															bind:value={selectedSettingsAccountType}
															name="type"
															options={accountTypeOptions}
														/>
														<input
															name="openingBalance"
															type="text"
															inputmode="decimal"
													value={amountFromCents(account.openingBalance)}
															placeholder="Opening"
															on:input={formatAmountInput}
														/>
													</div>
													<label>
														FX markup (%)
														<input name="fxMarkupPercent" type="number" min="0" max="100" step="0.1" value={account.fxMarkupPercent} inputmode="decimal" />
													</label>
													<input name="icon" value={account.icon} placeholder="Emoji icon (e.g. 🏦)" />
													<input name="color" type="color" value={account.color} title="Account color" />
													<label class="account-status-toggle">
														<input name="inactive" type="checkbox" checked={!isActive(account)} />
														<span>
															<strong>Inactive account</strong>
															<small>Keep its history, but prevent it from being selected for new transactions.</small>
														</span>
													</label>
													<div class="desktop-inline-wizard-actions">
														<button type="submit">Save account</button>
														<button class="ghost" type="button" on:click={() => (selectedSettingsAccountId = '')}>Cancel</button>
													</div>
												</form>
											{/if}
										{/snippet}
									</Collapsible.Content>
								</Collapsible.Root>
							{/each}
						</div>
						{#if desktopAddAccountWizardOpen}
							<form class="desktop-form-grid desktop-inline-wizard" on:submit|preventDefault={submitDesktopAccountWizard}>
								<input name="name" placeholder="Account name" required />
								<div class="field-grid">
									<AppSelect ariaLabel="Account type" bind:value={accountFormType} name="type" options={accountTypeOptions} />
									<input name="openingBalance" type="text" inputmode="decimal" placeholder="Opening" on:input={formatAmountInput} />
								</div>
								<label>
									FX markup (%)
									<input name="fxMarkupPercent" type="number" min="0" max="100" step="0.1" value="3.5" inputmode="decimal" />
								</label>
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
							{#each allCategories as category}
								<Collapsible.Root
									open={selectedSettingsCategoryId === category.id}
									onOpenChange={(open) => {
										selectedSettingsCategoryId = open ? category.id : '';
										if (open) desktopAddCategoryWizardOpen = false;
									}}
								>
									<Collapsible.Trigger
										class={`desktop-account-row${isActive(category) ? '' : ' inactive-category'}`}
									>
										<div class="entity-lead">
											<span class="entity-icon">{categoryEmoji(category)}</span>
											<span style={`--swatch:${category.color}`}></span>
										</div>
										<div>
											<strong>{category.name}</strong>
											<small>
												{categoryScopeLabel(category.scope)} · {categoryTypeLabel(category.type)} · Target
												{currency(category.monthlyTarget)}{isActive(category) ? '' : ' · Inactive'}
											</small>
										</div>
										<div class="desktop-category-row-summary">
											<b>{currency(currentMonthCategoryAmounts.get(category.id) ?? 0)}</b>
											<small>{selectedSettingsCategoryId === category.id ? 'Close' : 'This month · Edit'}</small>
										</div>
									</Collapsible.Trigger>
									<Collapsible.Content forceMount>
										{#snippet child({ props, open })}
											{#if open}
												<form
													{...props}
													class="desktop-form-grid desktop-inline-wizard desktop-row-editor"
													on:submit|preventDefault={submitCategoryUpdate}
													transition:slide={{ duration: 220 }}
												>
													<input name="name" value={category.name} placeholder="Category name" required />
													<div class="field-grid">
														<AppSelect
															ariaLabel="Category type"
															bind:value={selectedSettingsCategoryType}
															name="type"
															options={categoryTypeOptions}
														/>
														<AppSelect
															ariaLabel="Category scope"
															bind:value={selectedSettingsCategoryScope}
															name="scope"
															options={categoryScopeOptions}
														/>
													</div>
													<div class="field-grid">
														<input
															name="monthlyTarget"
															type="text"
															inputmode="decimal"
													value={amountFromCents(category.monthlyTarget)}
															placeholder="Monthly target"
															on:input={formatAmountInput}
														/>
													</div>
													<input name="icon" value={category.icon} placeholder="Emoji icon (e.g. 🛒)" />
													<input name="color" type="color" value={category.color} title="Category color" />
													<label class="account-status-toggle">
														<input name="inactive" type="checkbox" checked={!isActive(category)} />
														<span>
															<strong>Inactive category</strong>
															<small>Keep its history, but prevent it from being selected for new transactions.</small>
														</span>
													</label>
													<div class="desktop-inline-wizard-actions">
														<button type="submit">Save category</button>
														<button class="ghost" type="button" on:click={() => (selectedSettingsCategoryId = '')}>Cancel</button>
													</div>
												</form>
											{/if}
										{/snippet}
									</Collapsible.Content>
								</Collapsible.Root>
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
								<strong>{currency(desktopReviewTotal)}</strong>
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
									<b>{formatSignedCurrency(summary.netCashFlow, baseCurrency)}</b>
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
						<div class="field-grid">
							<label>
								Amount
								<input name="amount" type="text" inputmode="decimal" placeholder="0.00" on:input={formatAmountInput} required />
							</label>
							<label>
								Currency
								<AppSelect
									ariaLabel="Transaction currency"
									bind:value={selectedCurrency}
									name="currency"
									options={currencyOptions}
									required
								/>
							</label>
						</div>
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
									disabled={accounts.length === 0}
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
											<ChevronDown size={18} />
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
			{:else if desktopScreen === 'statements'}
				<section class="desktop-statement-page">
					{#if activeGroup}
						<StatementIngestion groupId={activeGroup.id} {accounts} {categories} entries={entries} onConfirmed={() => finance.syncNow()} />
					{/if}
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
						<button
							type="button"
							disabled={$syncStatus.state === 'syncing'}
							aria-busy={$syncStatus.state === 'syncing'}
							on:click={handleManualSync}
						>
							<RefreshCw size={16} class={$syncStatus.state === 'syncing' ? 'spinning' : ''} />
							{$syncStatus.state === 'syncing' ? 'Syncing...' : 'Sync now'}
						</button>
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

<script lang="ts">
	import { ArrowDownRight, ArrowUpRight, BarChart3, Coffee, Lightbulb, Sparkles, Target, TrendingUp } from 'lucide-svelte';
	import { tick } from 'svelte';
	import type { Category, LedgerEntry } from '$lib/types';
	import { buildMonthlySpending, buildMonthDetail, buildSpendingInsights } from '$lib/analytics';
	import { currency, todayInputValue } from '$lib/utils';

	export let categories: Category[] = [];
	export let entries: LedgerEntry[] = [];
	export let baseCurrency = 'SGD';
	export let asOfDate = todayInputValue();

	let months = 6;
	let scope = 'all';
	let categoryId = 'all';
	let selectedMonth = asOfDate.slice(0, 7);
	let chartScroller: HTMLDivElement | undefined;
	$: scopeCategories = categories.filter((category) => scope === 'all' || (category.scope === 'user' ? 'personal' : 'household') === scope);
	$: if (categoryId !== 'all' && !scopeCategories.some((category) => category.id === categoryId)) categoryId = 'all';
	$: filteredCategories = scopeCategories.filter((category) => categoryId === 'all' || category.id === categoryId);
	$: series = buildMonthlySpending(filteredCategories, entries, asOfDate, months, baseCurrency);
	$: if (!series.some((month) => month.key === selectedMonth)) selectedMonth = asOfDate.slice(0, 7);
	$: detail = buildMonthDetail(filteredCategories, entries, selectedMonth, asOfDate, baseCurrency);
	$: insights = buildSpendingInsights(filteredCategories, entries, selectedMonth, asOfDate, baseCurrency);
	$: hasData = series.some((month) => month.count > 0 || month.pendingCount > 0);
	$: chartMax = niceMaximum(Math.max(1, ...series.map((month) => month.spent)));
	$: hasComparison = detail.current.length > 0 && detail.previous.length > 0 && detail.pendingCount === 0 && detail.previousPendingCount === 0;
	$: comparisonLabel = detail.isCurrent
		? `vs ${new Intl.DateTimeFormat(undefined, { month: 'short' }).format(new Date(`${detail.previousKey}-01T00:00:00`))} 1–${detail.previousCutoff}, ${detail.previousKey.slice(0, 4)}`
		: `vs ${monthLabel(detail.previousKey)}`;
	$: selectedCategoryName = categories.find((category) => category.id === categoryId)?.name;
	$: keepSelectedMonthVisible(chartScroller, selectedMonth, months);

	async function keepSelectedMonthVisible(scroller: HTMLDivElement | undefined, _selectedMonth: string, _months: number) {
		if (!scroller) return;
		await tick();
		if (!scroller.isConnected || scroller.scrollWidth <= scroller.clientWidth) return;
		const selected = scroller.querySelector<HTMLButtonElement>('[aria-pressed="true"]');
		if (!selected) return;
		const bar = selected.getBoundingClientRect();
		const container = scroller.getBoundingClientRect();
		scroller.scrollLeft += bar.left - container.left - (container.width - bar.width) / 2;
	}

	function monthLabel(key: string): string {
		return new Intl.DateTimeFormat(undefined, { month: 'long', year: 'numeric' }).format(new Date(`${key}-01T00:00:00`));
	}
	function money(amount: number): string { return currency(Math.round(amount), baseCurrency); }
	function axisMoney(amount: number): string {
		return new Intl.NumberFormat(undefined, { maximumFractionDigits: 1, notation: 'compact' }).format(amount / 100);
	}
	function niceMaximum(amount: number): number {
		const magnitude = 10 ** Math.floor(Math.log10(amount));
		return Math.ceil(amount / magnitude / 0.5) * magnitude * 0.5;
	}
</script>

<div class="home-analytics">
	<section class="analytics-card trend-card" aria-label="Monthly spending analytics">
		<div class="analytics-heading">
			<div><span class="eyebrow"><BarChart3 size={14} /> THE BIGGER PICTURE</span><h2>Spending over time</h2></div>
			<div class="range-toggle" role="group" aria-label="Chart time range">
				{#each [6, 12] as range}
					<button type="button" class:active={months === range} aria-pressed={months === range} on:click={() => months = range}>{range}M</button>
				{/each}
			</div>
		</div>
		<div class="analytics-filters">
			<label><span>Spending scope</span><select aria-label="Analytics spending scope" bind:value={scope}><option value="all">All spending</option><option value="household">Household</option><option value="personal">Personal</option></select></label>
			<label><span>Category</span><select aria-label="Analytics category" bind:value={categoryId}><option value="all">All categories</option>{#each scopeCategories.filter((category) => category.type === 'expense') as category}<option value={category.id}>{category.name}{category.deletedAt ? ' · Inactive' : ''}</option>{/each}</select></label>
		</div>
		<div class="selected-summary" aria-live="polite">
			<div><span>{monthLabel(selectedMonth)}{detail.isCurrent ? ' · so far' : ''}</span><strong>{money(detail.spent)}</strong></div>
			{#if hasComparison}
				<div class="comparison" class:lower={detail.delta < 0} class:higher={detail.delta > 0}>
					<span>{#if detail.delta < 0}<ArrowDownRight size={15} />{:else if detail.delta > 0}<ArrowUpRight size={15} />{/if}{detail.percent === null ? money(Math.abs(detail.delta)) : `${Math.abs(detail.percent).toFixed(1)}%`} {detail.delta < 0 ? 'less' : detail.delta > 0 ? 'more' : 'change'}</span>
					<small>{comparisonLabel}</small>
				</div>
			{:else}<small class="comparison-note">{detail.pendingCount || detail.previousPendingCount ? 'Comparison available after currency conversion.' : detail.current.length === 0 ? 'No expenses recorded in this month.' : 'Add last month’s expenses to compare.'}</small>{/if}
		</div>
		{#if hasData}
			<div class="chart-caption"><span>Recorded expenses · {baseCurrency}</span><span><i></i> Current month is partial</span></div>
			<div class="chart-layout">
				<div class="chart-y-axis" aria-hidden="true">{#each [1, 0.5, 0] as fraction}<span>{axisMoney(chartMax * fraction)}</span>{/each}</div>
				<div class="chart-scroll" bind:this={chartScroller}>
					<div class="month-chart" style={`--months:${months}`} role="group" aria-label="Select a month to see its spending and insights">
						{#each series as point}
							<button class="month-column" type="button" class:selected={point.key === selectedMonth} aria-pressed={point.key === selectedMonth} aria-label={`${monthLabel(point.key)}: ${money(point.spent)}, ${point.count} recorded expenses${point.isCurrent ? ', month to date' : ''}${point.pendingCount ? `, ${point.pendingCount} awaiting currency conversion` : ''}`} title={`${monthLabel(point.key)}: ${money(point.spent)}${point.isCurrent ? ' so far' : ''}`} on:click={() => selectedMonth = point.key}>
								<span class="bar-track"><span class="month-bar" class:partial={point.isCurrent} class:zero={point.spent === 0} style={`--height:${point.spent / chartMax * 100}%`}></span></span>
								<span class="month-label">{point.label}<small>{point.key.slice(0, 4)}</small></span>
							</button>
						{/each}
					</div>
				</div>
			</div>
			<p class="chart-help">Select a month to explore. Empty months have no recorded expenses.</p>
		{:else}
			<div class="analytics-empty"><BarChart3 size={30} /><h3>Your spending story starts here</h3><p>Add expenses to see your monthly trend and find opportunities to save.</p></div>
		{/if}
		{#if detail.pendingCount > 0}<p class="conversion-note">{detail.pendingCount} expense{detail.pendingCount === 1 ? '' : 's'} awaiting currency conversion. Totals and insights update after sync.</p>{/if}
		{#if detail.current.length > 0}
			<div class="month-facts"><span><b>{detail.current.length}</b> expenses</span><span><b>{money(detail.spent / detail.current.length)}</b> average purchase</span></div>
			<div class="category-breakdown">
				{#each detail.categories.slice(0, 3) as category}
					<div class="category-line"><div><span class="category-dot" style={`background:${category.color}`}></span><span>{category.name}</span><b>{money(category.spent)}</b></div><div class="category-track"><span style={`width:${detail.spent > 0 ? category.spent / detail.spent * 100 : 0}%;background:${category.color}`}></span></div></div>
				{/each}
			</div>
		{/if}
		{#if hasData}
			<details class="chart-data"><summary>View monthly totals</summary><table><caption class="sr-only">Monthly recorded spending in {baseCurrency}</caption><thead><tr><th scope="col">Month</th><th scope="col">Expenses</th><th scope="col">Spent</th></tr></thead><tbody>{#each series as point}<tr><th scope="row">{monthLabel(point.key)}{point.isCurrent ? ' (so far)' : ''}</th><td>{point.count}</td><td>{money(point.spent)}{point.pendingCount ? ' + pending FX' : ''}</td></tr>{/each}</tbody></table></details>
		{/if}
	</section>

	<section class="analytics-card insights-card" aria-label="Spending insights">
		<div class="analytics-heading"><div><span class="eyebrow"><Sparkles size={14} /> SMALL CHANGES, MORE SAVINGS</span><h2>Spending insights</h2></div></div>
		<p class="insights-period">{monthLabel(selectedMonth)}{selectedCategoryName ? ` · ${selectedCategoryName}` : scope !== 'all' ? ` · ${scope === 'personal' ? 'Personal' : 'Household'}` : ''}</p>
		<div class="insights-list" aria-live="polite">
			{#each insights as insight (insight.id)}
				<article class="insight" class:positive={insight.kind === 'positive'}>
					<div class="insight-title"><span class="insight-icon">{#if insight.kind === 'coffee'}<Coffee size={18} />{:else if insight.kind === 'budget'}<Target size={18} />{:else if insight.kind === 'trend'}<TrendingUp size={18} />{:else}<Lightbulb size={18} />{/if}</span><h3>{insight.title}</h3></div>
					<p>{insight.description}</p><small>{insight.context}</small>
					{#if insight.saving}<span class="saving-chip">{money(insight.saving)} possible saving</span>{/if}
				</article>
			{:else}
				<div class="insight-empty"><span class="insight-icon"><Lightbulb size={24} /></span><h3>{detail.pendingCount ? 'Waiting for currency conversion' : detail.current.length < 3 ? 'A few more expenses will help' : 'Nothing stands out yet'}</h3><p>{detail.pendingCount ? 'Sync to convert foreign expenses before looking for patterns.' : detail.current.length < 3 ? 'Keep logging your purchases. We’ll look for repeat stops, category increases, and spending close to your targets.' : 'As patterns emerge, you’ll see practical suggestions here. Set category targets in Settings for budget insights.'}</p></div>
			{/each}
		</div>
		<p class="insight-method"><Sparkles size={13} /><span>Pattern-based · calculated on your device.<br />Suggestions use recorded expenses; savings are optional estimates.</span></p>
	</section>
</div>

<style>
	.home-analytics { display: grid; grid-column: 1 / -1; gap: 1rem; min-width: 0; margin: 0 0 1rem; }
	.analytics-card { background: var(--panel, #fff); border: 1px solid var(--line, #e5e7eb); border-radius: var(--radius-md, 16px); color: var(--ink, #11182b); min-width: 0; padding: 1.15rem; }
	.analytics-heading { display: flex; align-items: center; justify-content: space-between; gap: .6rem; }
	.eyebrow { display: flex; align-items: center; gap: .35rem; color: var(--muted, #596175); font-size: .6rem; font-weight: 800; letter-spacing: .065em; }
	h2 { font-size: 1.2rem; letter-spacing: -.025em; margin: .4rem 0 0; }
	.range-toggle { display: flex; background: var(--field, #f4f5f8); border: 1px solid var(--line, #e5e7eb); border-radius: 8px; padding: 3px; flex-shrink: 0; }
	.range-toggle button { background: transparent; color: var(--muted, #596175); border-radius: 5px; font-size: .75rem; font-weight: 800; min-height: 2.15rem; min-width: 2.6rem; }
	.range-toggle button.active { background: var(--primary, #2563eb); color: var(--paper, white); }
	.analytics-filters { display: grid; grid-template-columns: 1fr 1fr; gap: .6rem; margin: 1rem 0; }
	.analytics-filters label { min-width: 0; gap: .35rem; font-size: .68rem; }
	.analytics-filters select { min-width: 0; min-height: 2.55rem; border: 1px solid var(--line, #e5e7eb); border-radius: 8px; outline: 0; padding: .5rem; font-weight: 600; }
	.selected-summary { display: flex; align-items: center; justify-content: space-between; gap: .8rem; min-height: 4.4rem; margin: 1rem 0 1.3rem; }
	.selected-summary > div:first-child > span { display: block; font-size: .78rem; color: var(--muted, #596175); }
	.selected-summary strong { display: block; font-size: clamp(1.65rem, 2.5vw, 2.15rem); letter-spacing: -.045em; margin-top: .35rem; overflow-wrap: anywhere; font-variant-numeric: tabular-nums; }
	.comparison { text-align: right; flex-shrink: 0; }
	.comparison > span { display: inline-flex; align-items: center; gap: .2rem; font-size: .75rem; font-weight: 800; background: var(--field, #f4f5f8); border-radius: 6px; padding: .35rem .45rem; }
	.comparison.higher > span { background: color-mix(in srgb, #e47740 12%, var(--panel, white)); color: #a34518; }
	.comparison.lower > span { background: color-mix(in srgb, #10b981 12%, var(--panel, white)); color: #087c57; }
	.comparison small { display: block; font-size: .65rem; margin-top: .4rem; color: var(--muted, #596175); }
	.comparison-note { color: var(--muted, #596175); font-size: .7rem; line-height: 1.5; max-width: 9rem; text-align: right; }
	.chart-caption { display: flex; justify-content: space-between; flex-wrap: wrap; gap: .4rem; color: var(--muted, #596175); font-size: .65rem; margin-bottom: .75rem; }
	.chart-caption > span:last-child { display: flex; align-items: center; gap: .3rem; }
	.chart-caption i { width: .55rem; height: .55rem; border: 1px dashed var(--primary, #2563eb); display: inline-block; }
	.chart-layout { display: grid; grid-template-columns: 2.5rem minmax(0, 1fr); }
	.chart-y-axis { display: flex; flex-direction: column; justify-content: space-between; height: 160px; color: var(--muted, #596175); font-size: .63rem; padding-right: .3rem; }
	.chart-scroll { min-width: 0; overflow-x: auto; padding: 3px 3px 0; }
	.month-chart { display: grid; grid-template-columns: repeat(var(--months), minmax(0, 1fr)); gap: .5rem; min-width: calc(var(--months) * 33px); }
	.month-column { display: flex; flex-direction: column; justify-content: flex-end; background: transparent; padding: 0 .15rem; min-width: 0; border-radius: 5px; color: var(--muted, #596175); }
	.bar-track { width: 100%; height: 160px; display: flex; align-items: flex-end; justify-content: center; border-bottom: 1px solid var(--line, #e5e7eb); background: linear-gradient(to bottom, var(--line, #e5e7eb) 1px, transparent 1px); background-size: 100% 50%; }
	.month-bar { width: min(100%, 42px); height: var(--height); background: color-mix(in srgb, var(--primary, #2563eb) 35%, var(--panel, white)); border-radius: 5px 5px 0 0; transition: background 140ms; min-height: 2px; }
	.month-bar.partial { background: repeating-linear-gradient(135deg, transparent 0 4px, color-mix(in srgb, var(--primary, #2563eb) 20%, transparent) 4px 6px); border: 1px dashed var(--primary, #2563eb); border-bottom: 0; }
	.month-bar.zero { height: 2px; min-height: 2px; background: var(--line, #e5e7eb); border: 0; }
	.month-column.selected .month-bar:not(.partial):not(.zero), .month-column:hover .month-bar:not(.partial):not(.zero) { background: var(--primary, #2563eb); }
	.month-column.selected .month-bar.partial { background-color: color-mix(in srgb, var(--primary, #2563eb) 22%, var(--panel, white)); }
	.month-label { display: block; padding: .55rem 0 .25rem; font-size: .7rem; text-align: center; width: 100%; }
	.month-label small { display: block; font-size: .58rem; opacity: .65; margin-top: .15rem; }
	.month-column.selected .month-label { color: var(--primary, #2563eb); font-weight: 800; }
	button:focus-visible, select:focus-visible, summary:focus-visible { outline: 2px solid var(--primary, #2563eb); outline-offset: 2px; }
	.chart-help { font-size: .65rem; color: var(--muted, #596175); margin: .65rem 0 1rem; line-height: 1.5; }
	.month-facts { display: flex; flex-wrap: wrap; gap: .6rem 1.25rem; border-top: 1px solid var(--line, #e5e7eb); padding-top: .85rem; font-size: .72rem; color: var(--muted, #596175); }
	.month-facts b { color: var(--ink, #11182b); }
	.category-breakdown { display: grid; gap: .75rem; margin-top: 1rem; }
	.category-line > div:first-child { display: flex; align-items: center; gap: .4rem; font-size: .72rem; }
	.category-dot { width: .5rem; height: .5rem; border-radius: 50%; flex-shrink: 0; }
	.category-line b { margin-left: auto; font-variant-numeric: tabular-nums; white-space: nowrap; }
	.category-track { height: 4px; background: var(--field, #f4f5f8); border-radius: 4px; margin-top: .4rem; overflow: hidden; }
	.category-track span { display: block; height: 100%; border-radius: 4px; }
	.chart-data { border-top: 1px solid var(--line, #e5e7eb); margin-top: 1rem; padding-top: .7rem; font-size: .7rem; }
	.chart-data summary { cursor: pointer; color: var(--muted, #596175); }
	.chart-data table { width: 100%; border-collapse: collapse; margin-top: .7rem; font-size: .7rem; }
	.chart-data th, .chart-data td { text-align: right; padding: .45rem .2rem; border-bottom: 1px solid var(--line, #e5e7eb); }
	.chart-data th:first-child { text-align: left; }
	.insights-card { display: flex; flex-direction: column; background: linear-gradient(160deg, color-mix(in srgb, var(--primary, #2563eb) 4%, var(--panel, white)), var(--panel, white) 65%); }
	.insights-period { margin: .65rem 0 1rem; color: var(--muted, #596175); font-size: .72rem; }
	.insights-list { display: grid; gap: .8rem; }
	.insight { border: 1px solid var(--line, #e5e7eb); background: var(--panel, white); border-radius: 10px; padding: .9rem; }
	.insight-title { display: flex; align-items: center; gap: .65rem; }
	.insight-icon { display: inline-flex; align-items: center; justify-content: center; flex-shrink: 0; color: var(--primary, #2563eb); background: color-mix(in srgb, var(--primary, #2563eb) 10%, var(--panel, white)); width: 2rem; height: 2rem; border-radius: 9px; }
	h3 { font-size: .85rem; line-height: 1.4; margin: 0; }
	.insight p, .insight-empty p { color: var(--muted, #596175); font-size: .77rem; line-height: 1.65; margin: .65rem 0; }
	.insight small { display: block; color: var(--muted, #596175); font-size: .67rem; line-height: 1.5; }
	.saving-chip { display: inline-block; margin-top: .7rem; padding: .3rem .5rem; border-radius: 5px; background: color-mix(in srgb, #10b981 12%, var(--panel, white)); color: #087c57; font-size: .66rem; font-weight: 800; }
	.insight-method { display: flex; align-items: flex-start; gap: .4rem; color: var(--muted, #596175); font-size: .62rem; line-height: 1.6; margin: auto 0 0; padding-top: 1.15rem; }
	.insight-method :global(svg) { flex-shrink: 0; margin-top: .15rem; }
	.insight-empty { padding: 1rem .25rem; }
	.insight-empty .insight-icon { width: 2.75rem; height: 2.75rem; margin-bottom: .8rem; }
	.analytics-empty { display: flex; flex-direction: column; align-items: center; justify-content: center; text-align: center; min-height: 13rem; color: var(--muted, #596175); background: var(--field, #f4f5f8); border-radius: 10px; padding: 1.5rem; }
	.analytics-empty h3 { color: var(--ink, #11182b); margin-top: .8rem; }
	.analytics-empty p { font-size: .77rem; line-height: 1.6; max-width: 22rem; margin-bottom: 0; }
	.conversion-note { color: var(--muted, #596175); background: var(--field, #f4f5f8); border-radius: 8px; font-size: .72rem; line-height: 1.5; padding: .75rem; }
	:global([data-theme='dark']) .comparison.higher > span { color: #ffb88c; }
	:global([data-theme='dark']) .comparison.lower > span, :global([data-theme='dark']) .saving-chip { color: #6ee7b7; }
	@media (min-width: 1120px) { .home-analytics { grid-template-columns: minmax(0, 1.7fr) minmax(19rem, 1fr); margin-bottom: 0; } .analytics-card { padding: 1.35rem; } .chart-y-axis, .bar-track { height: 185px; } }
	@media (max-width: 360px) { .analytics-card { padding: .9rem; } .analytics-filters { grid-template-columns: 1fr; } .eyebrow { font-size: .55rem; letter-spacing: .025em; } }
</style>

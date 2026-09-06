<script lang="ts">
	import { CheckCircle2, Link2, Plus, Search, Trash2 } from 'lucide-svelte';
	import type { Account, Category, LedgerEntry, StatementIngestionRow } from '$lib/types';

	export let row: StatementIngestionRow;
	export let entries: LedgerEntry[] = [];
	export let accounts: Account[] = [];
	export let categories: Category[] = [];
	export let removing = false;
	export let onChooseMatch: (entryId: string) => void;
	export let onChooseNew: () => void;
	export let onRemove: () => void | Promise<void>;

	let query = '';
	let previousRowId = '';

	$: if (row.id !== previousRowId) {
		previousRowId = row.id;
		query = '';
	}
	$: candidateId = row.matchExpenseId || row.suggestedExpenseId || '';
	$: candidate = entries.find((entry) => entry.id === candidateId);
	$: results = searchEntries(query, entries);

	function searchEntries(value: string, source: LedgerEntry[]): LedgerEntry[] {
		const needle = value.trim().toLowerCase();
		if (!needle) return [];
		return source.filter((entry) => {
			const account = accounts.find((item) => item.id === entry.accountId)?.name ?? '';
			const category = categories.find((item) => item.id === entry.categoryId)?.name ?? '';
			const amount = (entry.amount / 100).toFixed(2);
			return `${entry.occurredOn} ${entry.merchant} ${amount} ${entry.currency} ${account} ${category}`.toLowerCase().includes(needle);
		}).slice(0, 8);
	}

	function accountName(id?: string | null): string { return accounts.find((item) => item.id === id)?.name ?? 'Unknown account'; }
	function categoryName(id?: string | null): string { return categories.find((item) => item.id === id)?.name ?? 'Uncategorised'; }
	function money(cents: number, currency = 'SGD'): string { return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(cents / 100); }
	function shortDate(value?: string | null): string {
		if (!value) return 'Date unavailable';
		return new Intl.DateTimeFormat(undefined, { day: 'numeric', month: 'short', year: 'numeric' }).format(new Date(`${value}T00:00:00`));
	}
</script>

<section class="match-review" aria-label="Match review">
	<header>
		<div><strong>Match review</strong><span>Compare the statement row with the master transaction before deciding.</span></div>
		{#if candidate}
			<span class="candidate-label"><Link2 size={15} />{row.matchExpenseId ? 'Selected master transaction' : `Suggested match${row.matchConfidence != null ? ` · ${Math.round(row.matchConfidence * 100)}%` : ''}`}</span>
		{/if}
	</header>

	<div class="comparison">
		<article>
			<p>Statement transaction</p>
			<strong>{row.merchant || 'Unreadable merchant'}</strong>
			<div><span>{shortDate(row.occurredOn)}</span><b>{money(row.amount, row.currency)}</b></div>
			<small>{accountName(row.accountId)} · {categoryName(row.categoryId)}</small>
		</article>
		<article class:candidate-empty={!candidate}>
			<p>Master transaction</p>
			{#if candidate}
				<strong>{candidate.merchant}</strong>
				<div><span>{shortDate(candidate.occurredOn)}</span><b>{money(candidate.amount, candidate.currency)}</b></div>
				<small>{accountName(candidate.accountId)} · {categoryName(candidate.categoryId)}</small>
			{:else}
				<strong>No match selected</strong>
				<small>Search the master table below to select one manually.</small>
			{/if}
		</article>
	</div>

	<div class="decision-actions">
		<button class:active={row.reviewStatus === 'matched'} type="button" disabled={!candidate} on:click={() => candidate && onChooseMatch(candidate.id)}><CheckCircle2 size={17} /> Confirm same transaction</button>
		<button class:active={row.reviewStatus === 'new'} type="button" on:click={onChooseNew}><Plus size={17} /> Create new transaction</button>
		<button class="remove-action" type="button" disabled={removing} on:click={() => void onRemove()}><Trash2 size={17} />{removing ? 'Removing…' : 'Remove from staging'}</button>
	</div>

	<div class="manual-match">
		<label><span>Manual match</span><div class="match-search"><Search size={17} /><input bind:value={query} type="search" placeholder="Search master transactions by merchant, date, amount, account…" aria-label="Search master transactions" /></div></label>
		{#if query.trim()}
			<div class="match-results">
				{#each results as entry (entry.id)}
					<button class:selected={row.matchExpenseId === entry.id} type="button" on:click={() => onChooseMatch(entry.id)}>
						<span><strong>{entry.merchant}</strong><small>{shortDate(entry.occurredOn)} · {accountName(entry.accountId)} · {categoryName(entry.categoryId)}</small></span>
						<b>{money(entry.amount, entry.currency)}</b>
					</button>
				{:else}<p>No master transactions match “{query.trim()}”.</p>{/each}
			</div>
		{/if}
	</div>
</section>

<style>
	.match-review{background:color-mix(in srgb,var(--field) 70%,transparent);border:1px solid var(--line);border-radius:14px;display:grid;gap:1rem;margin-top:1rem;padding:1rem}.match-review header{align-items:flex-start;display:flex;gap:1rem;justify-content:space-between}.match-review header>div{display:grid;gap:.25rem}.match-review header span,.match-review small{color:var(--muted);font-size:.78rem}.candidate-label{align-items:center;background:#6794ff21;border-radius:999px;color:var(--primary)!important;display:flex;flex:none;font-weight:800;gap:.35rem;padding:.4rem .6rem}.comparison{display:grid;gap:.75rem;grid-template-columns:repeat(2,minmax(0,1fr))}.comparison article{background:var(--panel);border:1px solid var(--line);border-radius:12px;display:grid;gap:.5rem;min-width:0;padding:.9rem}.comparison article.candidate-empty{border-style:dashed}.comparison p{color:var(--muted);font-size:.7rem;font-weight:850;letter-spacing:.06em;text-transform:uppercase}.comparison strong,.comparison small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.comparison article>div{align-items:center;display:flex;gap:.7rem;justify-content:space-between}.comparison b{font-size:1rem}.decision-actions{display:grid;gap:.65rem;grid-template-columns:repeat(3,minmax(0,1fr))}.decision-actions button{align-items:center;background:var(--panel);border:1px solid var(--line);border-radius:11px;color:var(--ink);display:flex;font-weight:800;gap:.45rem;justify-content:center;min-height:2.8rem;padding:.65rem .8rem}.decision-actions button.active{background:#3dccaa20;border-color:#3dccaa70;color:#3dccaa}.decision-actions .remove-action{color:#ef8f87}.manual-match{display:grid;gap:.55rem}.manual-match label{display:grid;gap:.4rem}.manual-match label>span{color:var(--muted);font-size:.75rem;font-weight:800}.match-search{align-items:center;background:var(--panel);border:1px solid var(--line);border-radius:10px;color:var(--muted);display:flex;gap:.45rem;padding:0 .7rem}.match-search input{background:transparent;border:0;color:var(--ink);min-height:2.7rem;outline:0;width:100%}.match-results{background:var(--panel);border:1px solid var(--line);border-radius:11px;display:grid;max-height:19rem;overflow:auto}.match-results button{align-items:center;background:transparent;border:0;border-bottom:1px solid var(--line);color:var(--ink);display:flex;gap:1rem;justify-content:space-between;padding:.7rem .8rem;text-align:left;width:100%!important}.match-results button:last-child{border-bottom:0}.match-results button:hover,.match-results button.selected{background:var(--panel-hover)}.match-results button.selected{box-shadow:inset 3px 0 var(--primary)}.match-results button>span{display:grid;gap:.2rem;min-width:0}.match-results strong,.match-results small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.match-results b{flex:none}.match-results p{color:var(--muted);font-size:.85rem;margin:0;padding:.9rem}
	@media(max-width:767px){.match-review header{display:grid}.candidate-label{justify-self:start}.comparison{grid-template-columns:1fr}.decision-actions{grid-template-columns:1fr}.decision-actions button{justify-content:flex-start}.comparison strong,.comparison small{white-space:normal}}
</style>

<script lang="ts">
	import { onMount } from 'svelte';
	import { currency } from '$lib/utils';
	import { sameSyncRecord, type SyncConflict, type SyncRecord, type SyncResolution } from '$lib/sync-protocol';
	import type { FinanceState } from '$lib/types';

	export let conflict: SyncConflict;
	export let state: FinanceState | null;
	export let onChoose: (choice: SyncResolution) => void;
	let dialog: HTMLDialogElement;
	const fields: Record<string, string> = {
		name: 'Name', merchant: 'Merchant', amount: 'Amount', currency: 'Currency', occurredOn: 'Date',
		accountId: 'Account', categoryId: 'Category', type: 'Type', scope: 'Visibility', note: 'Note',
		openingBalance: 'Opening balance', monthlyTarget: 'Monthly target', fxMarkupPercent: 'FX markup',
		color: 'Colour', icon: 'Icon', metadata: 'Details', deletedAt: 'Status'
	};

	function value(record: SyncRecord | null, key: string): unknown {
		if (!record) return undefined;
		if (key === 'deletedAt') return record.deletedAt ? 'Deleted' : 'Active';
		return (record as unknown as Record<string, unknown>)[key];
	}
	function display(record: SyncRecord | null, key: string): string {
		if (!record) return 'Removed from server';
		const raw = value(record, key);
		if (raw == null || raw === '') return '—';
		if (['amount', 'openingBalance', 'monthlyTarget'].includes(key) && typeof raw === 'number') {
			return currency(raw, 'currency' in record ? record.currency : state?.settings.baseCurrency ?? 'SGD');
		}
		if (key === 'accountId') return state?.accounts.find(account => account.id === raw)?.name ?? String(raw);
		if (key === 'categoryId') return state?.categories.find(category => category.id === raw)?.name ?? String(raw);
		if (key === 'fxMarkupPercent') return `${raw}%`;
		return typeof raw === 'object' ? JSON.stringify(raw, null, 2) : String(raw);
	}
	$: title = 'merchant' in conflict.local ? conflict.local.merchant : 'name' in conflict.local ? conflict.local.name : 'Category adjustment';
	$: changedFields = Object.keys(fields).filter(key => !sameSyncRecord(value(conflict.local, key), value(conflict.server, key)));
	onMount(() => { dialog.showModal(); });
</script>

<dialog bind:this={dialog} class="sync-conflict-dialog" aria-labelledby="sync-conflict-title" aria-describedby="sync-conflict-description"
	on:cancel|preventDefault={() => onChoose('later')}>
	<header>
		<span class="eyebrow">Sync conflict</span>
		<h2 id="sync-conflict-title">Which version should we keep?</h2>
		<p id="sync-conflict-description">{title || 'This record'} changed on the server while you had local edits. Choose a version to continue.</p>
	</header>
	<div class="versions-heading"><strong>Server version</strong><strong>Your local version</strong></div>
	<div class="conflict-fields">
		{#each changedFields as key}
			<section>
				<h3>{fields[key]}</h3>
				<div class="version-values"><p>{display(conflict.server, key)}</p><p>{display(conflict.local, key)}</p></div>
			</section>
		{/each}
		{#if !changedFields.length}<p>The record has changed since this device last synced.</p>{/if}
	</div>
	<footer>
		<button type="button" class="keep-server" on:click={() => onChoose('server')}>Keep server version</button>
		<button type="button" class="keep-local" on:click={() => onChoose('local')}>Keep my local version</button>
		<button type="button" class="decide-later" on:click={() => onChoose('later')}>Decide later</button>
	</footer>
	<p class="conflict-help">Keeping your local version saves it to the server. Decide later keeps your edit on this device.</p>
</dialog>

<style>
	.sync-conflict-dialog { background: var(--panel); color: var(--ink); border: 1px solid var(--line); border-radius: 20px; padding: 1.4rem; width: min(42rem, calc(100vw - 1.5rem)); max-height: calc(100dvh - 2rem); overflow-y: auto; box-shadow: 0 20px 80px #0005; }
	.sync-conflict-dialog::backdrop { background: #0009; }
	header h2 { font-size: 1.4rem; margin: .5rem 0; }
	header p, .conflict-help { color: var(--muted); line-height: 1.5; }
	.eyebrow { color: var(--primary); font-weight: 800; font-size: .8rem; text-transform: uppercase; letter-spacing: .07em; }
	.versions-heading, .version-values { display: grid; grid-template-columns: minmax(0,1fr) minmax(0,1fr); gap: .8rem; }
	.versions-heading { margin: 1.25rem 0 .6rem; font-size: .9rem; }
	.conflict-fields { border: 1px solid var(--line); border-radius: 12px; }
	.conflict-fields section { padding: .7rem; border-bottom: 1px solid var(--line); }
	.conflict-fields section:last-child { border-bottom: 0; }
	.conflict-fields h3 { font-size: .75rem; color: var(--muted); margin: 0 0 .35rem; }
	.version-values p { margin: 0; font-size: .9rem; white-space: pre-wrap; overflow-wrap: anywhere; max-height: 12rem; overflow-y: auto; }
	footer { display: grid; grid-template-columns: 1fr 1fr; gap: .6rem; margin-top: 1.2rem; }
	footer button { padding: .8rem .6rem; border-radius: 10px; width: 100%; }
	.keep-local, .decide-later { background: var(--field); border: 1px solid var(--line); color: var(--ink); }
	.decide-later { grid-column: 1 / -1; }
	.conflict-help { font-size: .8rem; margin: .8rem 0 0; }
	@media (max-width: 430px) { .sync-conflict-dialog { padding: 1rem; } footer { grid-template-columns: 1fr; } .decide-later { grid-column: auto; } }
</style>

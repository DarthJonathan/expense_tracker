<script lang="ts">
	import { onMount } from 'svelte';
	import { deleteLocalStatementJob, getLocalStatementJobs, putLocalStatementJob } from '$lib/db';
	import {
		confirmStatementIngestion,
		createStatementIngestion,
		deleteStatementIngestion,
		extractEmbeddedPdfText,
		getStatementIngestion,
		listStatementIngestions,
		parseDbsStatementText,
		updateStatementRow
	} from '$lib/statements';
	import type {
		Account,
		Category,
		LedgerEntry,
		LocalStatementJob,
		StatementIngestion,
		StatementIngestionDetail,
		StatementIngestionRow,
		StatementReviewStatus
	} from '$lib/types';
	import type { ParsedStatement } from '$lib/statements';
	import { isoNow, makeId } from '$lib/utils';

	export let groupId: string;
	export let accounts: Account[] = [];
	export let categories: Category[] = [];
	export let entries: LedgerEntry[] = [];
	export let onConfirmed: (() => unknown | Promise<unknown>) | undefined = undefined;

	let selectedAccountId = '';
	let selectedFile: File | null = null;
	let localJobs: LocalStatementJob[] = [];
	let ingestions: StatementIngestion[] = [];
	let detail: StatementIngestionDetail | null = null;
	let loading = false;
	let actionError = '';
	let actionMessage = '';
	let fileInput: HTMLInputElement | null = null;

	$: if (!selectedAccountId && accounts.length) selectedAccountId = accounts[0].id;

	onMount(() => {
		void refresh();
	});

	async function refresh(): Promise<void> {
		if (!groupId) return;
		loading = true;
		actionError = '';
		try {
			const [nextJobs, nextIngestions] = await Promise.all([getLocalStatementJobs(groupId), listStatementIngestions(groupId)]);
			localJobs = nextJobs;
			ingestions = nextIngestions;
			if (detail && !nextIngestions.some((item) => item.id === detail?.ingestion.id)) detail = null;
		} catch (error) {
			actionError = messageFor(error);
		} finally {
			loading = false;
		}
	}

	async function queueFile(): Promise<void> {
		if (!selectedFile || !selectedAccountId) return;
		const file = selectedFile;
		actionError = '';
		if (!file.name.toLowerCase().endsWith('.pdf') && file.type !== 'application/pdf') {
			actionError = 'Choose a PDF statement.';
			return;
		}
		if (file.size > 25 * 1024 * 1024) {
			actionError = 'The local PDF limit is 25 MB.';
			return;
		}
		try {
			const job: LocalStatementJob = {
				id: makeId(),
				groupId,
				accountId: selectedAccountId,
				file,
				fileName: file.name,
				fileType: file.type,
				sourceFingerprint: await sha256Fingerprint(file),
				status: 'local_parsing',
				stage: 'embedded_text',
				createdAt: isoNow(),
				updatedAt: isoNow()
			};
			await putLocalStatementJob(job);
			localJobs = [job, ...localJobs];
			selectedFile = null;
			if (fileInput) fileInput.value = '';
			await resumeLocalJob(job);
		} catch (error) {
			actionError = messageFor(error);
		}
	}

	async function resumeLocalJob(job: LocalStatementJob): Promise<void> {
		actionError = '';
		actionMessage = '';
		let working: LocalStatementJob = {
			...job,
			sourceFingerprint: job.sourceFingerprint || (await sha256Fingerprint(job.file)),
			status: 'local_parsing',
			error: undefined,
			updatedAt: isoNow()
		};
		await checkpoint(working);
		try {
			let parsed = working.parsedPayload as unknown as ParsedStatement | undefined;
			if (!parsed) {
				working = { ...working, stage: 'embedded_text', updatedAt: isoNow() };
				await checkpoint(working);
				const text = await extractEmbeddedPdfText(working.file);
				parsed = parseDbsStatementText(text, working.accountId, working.fileName);
			}
			parsed.clientRequestId = working.id;
			parsed.sourceFingerprint = working.sourceFingerprint || undefined;
			if (parsed.rows.length === 0) throw new Error('No statement transactions could be recognized from the local text layer.');
			working = { ...working, stage: 'structured_ready', parsedPayload: parsed as unknown as Record<string, unknown>, updatedAt: isoNow() };
			await checkpoint(working);
			working = { ...working, stage: 'saving_structured', updatedAt: isoNow() };
			await checkpoint(working);
			const created = await createStatementIngestion(groupId, parsed);
			await deleteLocalStatementJob(job.id);
			localJobs = localJobs.filter((item) => item.id !== job.id);
			detail = created;
			ingestions = [created.ingestion, ...ingestions.filter((item) => item.id !== created.ingestion.id)];
			actionMessage = `${created.rows.length} normalized rows are ready for shared review.`;
		} catch (error) {
			const failed: LocalStatementJob = {
				...working,
				status: 'failed',
				stage: working.stage === 'embedded_text' ? 'ocr' : working.stage,
				error: messageFor(error),
				updatedAt: isoNow()
			};
			await putLocalStatementJob(failed);
			localJobs = localJobs.map((item) => (item.id === failed.id ? failed : item));
			actionError = failed.error ?? 'Local parsing failed.';
		}
	}

	async function checkpoint(job: LocalStatementJob): Promise<void> {
		await putLocalStatementJob(job);
		localJobs = localJobs.map((item) => (item.id === job.id ? job : item));
	}

	async function sha256Fingerprint(file: Blob): Promise<string> {
		if (!globalThis.crypto?.subtle) return '';
		const digest = await globalThis.crypto.subtle.digest('SHA-256', await file.arrayBuffer());
		return [...new Uint8Array(digest)].map((value) => value.toString(16).padStart(2, '0')).join('');
	}

	async function discardLocalJob(job: LocalStatementJob): Promise<void> {
		if (!window.confirm(`Discard the local PDF checkpoint for ${job.fileName}?`)) return;
		await deleteLocalStatementJob(job.id);
		localJobs = localJobs.filter((item) => item.id !== job.id);
	}

	async function openIngestion(ingestion: StatementIngestion): Promise<void> {
		actionError = '';
		try {
			detail = await getStatementIngestion(groupId, ingestion.id);
		} catch (error) {
			actionError = messageFor(error);
		}
	}

	async function saveRow(row: StatementIngestionRow): Promise<void> {
		if (!detail) return;
		actionError = '';
		try {
			detail = await updateStatementRow(groupId, detail.ingestion.id, row.id, {
				occurredOn: row.occurredOn || '',
				merchant: row.merchant,
				amount: row.amount,
				currency: row.currency,
				foreignAmount: row.foreignAmount ?? undefined,
				foreignCurrency: row.foreignCurrency,
				statementKind: row.statementKind,
				type: row.type,
				accountId: row.accountId || '',
				categoryId: row.categoryId || '',
				note: row.note,
				reviewStatus: row.reviewStatus,
				matchExpenseId: row.matchExpenseId || '',
				warningCodes: row.warningCodes
			});
			upsertIngestion(detail.ingestion);
			actionMessage = `Saved review decision for ${row.merchant || row.sourceRowKey}.`;
		} catch (error) {
			actionError = messageFor(error);
		}
	}

	async function confirmIngestion(): Promise<void> {
		if (!detail || !window.confirm('Confirm all reviewed rows? New rows will be inserted and matched rows will update the selected transactions.')) return;
		try {
			detail = await confirmStatementIngestion(groupId, detail.ingestion.id);
			upsertIngestion(detail.ingestion);
			await onConfirmed?.();
			actionMessage = 'Statement ingestion confirmed.';
		} catch (error) {
			actionError = messageFor(error);
		}
	}

	async function remove(): Promise<void> {
		if (!detail || !window.confirm('Soft-delete this ingestion and its staging rows? Confirmed transactions will be retained.')) return;
		try {
			await deleteStatementIngestion(groupId, detail.ingestion.id);
			ingestions = ingestions.filter((item) => item.id !== detail?.ingestion.id);
			detail = null;
			actionMessage = 'Statement staging data deleted; confirmed transactions were kept.';
		} catch (error) {
			actionError = messageFor(error);
		}
	}

	function upsertIngestion(next: StatementIngestion): void {
		ingestions = [next, ...ingestions.filter((item) => item.id !== next.id)];
	}

	function chooseMatch(row: StatementIngestionRow, value: string): void {
		row.matchExpenseId = value || null;
		if (value) row.reviewStatus = 'matched';
	}

	function chooseDecision(row: StatementIngestionRow, value: string): void {
		row.reviewStatus = value as StatementReviewStatus;
		if (row.reviewStatus === 'matched' && !row.matchExpenseId && row.suggestedExpenseId) {
			row.matchExpenseId = row.suggestedExpenseId;
		} else if (row.reviewStatus !== 'matched') {
			row.matchExpenseId = null;
		}
	}

	function changeAmount(row: StatementIngestionRow, event: Event): void {
		const value = Number((event.currentTarget as HTMLInputElement).value);
		row.amount = Number.isFinite(value) ? Math.max(0, Math.round(value * 100)) : 0;
	}

	function visibleCategories(row: StatementIngestionRow): Category[] {
		return categories.filter((category) => category.type === row.type && !category.deletedAt);
	}

	function validationItems(validation: Record<string, unknown>): string[] {
		const output: string[] = [];
		const newTotal = validation.newTransactionsTotal as { pass?: boolean; actual?: number; expected?: number } | undefined;
		if (newTotal) output.push(`New transactions: ${newTotal.pass ? 'pass' : 'check'} (${money(newTotal.actual ?? 0)})`);
		const balance = validation.balanceReconciliation as { pass?: boolean; actualGrandTotal?: number; expectedGrandTotal?: number } | undefined;
		if (balance) output.push(`Previous balance + activity: ${balance.pass ? 'pass' : 'check'} (${money(balance.expectedGrandTotal ?? 0)} expected)`);
		const unreadable = validation.unreadableRows as unknown[] | undefined;
		if (unreadable?.length) output.push(`${unreadable.length} unreadable row${unreadable.length === 1 ? '' : 's'}`);
		const foreign = validation.foreignCurrencyRows as unknown[] | undefined;
		if (foreign?.length) output.push(`${foreign.length} foreign-currency row${foreign.length === 1 ? '' : 's'}`);
		const cardholders = validation.cardholders as Array<{ name?: string; actualRowCount?: number; actualSubtotal?: number; subtotalPass?: boolean }> | undefined;
		for (const cardholder of cardholders ?? []) {
			output.push(`${cardholder.name || 'Cardholder'}: ${cardholder.actualRowCount ?? 0} rows · ${money(cardholder.actualSubtotal ?? 0)}${cardholder.subtotalPass == null ? '' : cardholder.subtotalPass ? ' · pass' : ' · check'}`);
		}
		return output;
	}

	function money(cents: number): string {
		return new Intl.NumberFormat(undefined, { style: 'currency', currency: detail?.ingestion.statementCurrency ?? 'SGD' }).format(cents / 100);
	}

	function statusLabel(status: string): string {
		return status.replace(/_/g, ' ');
	}

	function messageFor(error: unknown): string {
		return error instanceof Error ? error.message : 'Unexpected statement ingestion error.';
	}
</script>

<section class="statement-ingestion" aria-label="Statement ingestion">
	<header>
		<div>
			<h2>Statement import</h2>
			<p>PDF files and page text stay on this device. Only normalized rows are sent for shared review. This first parser supports DBS text-layer statements.</p>
		</div>
		<button class="ghost" type="button" on:click={() => void refresh()} disabled={loading}>{loading ? 'Loading…' : 'Refresh'}</button>
	</header>

	<div class="upload-card">
		<label>
			Card account
			<select bind:value={selectedAccountId}>
				{#each accounts as account}<option value={account.id}>{account.name}</option>{/each}
			</select>
		</label>
		<label>
			PDF statement
			<input bind:this={fileInput} type="file" accept="application/pdf,.pdf" on:change={(event) => (selectedFile = (event.currentTarget.files ?? [])[0] ?? null)} />
		</label>
		<button type="button" disabled={!selectedFile || !selectedAccountId} on:click={() => void queueFile()}>Parse locally</button>
	</div>

	{#if actionError}<p class="statement-error" role="alert">{actionError}</p>{/if}
	{#if actionMessage}<p class="statement-success">{actionMessage}</p>{/if}

	{#if localJobs.length}
		<section class="local-jobs">
			<h3>On-device checkpoints</h3>
			{#each localJobs as job}
				<div class="job-row">
					<div><strong>{job.fileName}</strong><small>{statusLabel(job.status)} · {statusLabel(job.stage)}</small>{#if job.error}<small>{job.error}</small>{/if}</div>
					<div class="actions">
						<button type="button" on:click={() => void resumeLocalJob(job)}>Resume</button>
						<button class="danger ghost" type="button" on:click={() => void discardLocalJob(job)}>Discard local file</button>
					</div>
				</div>
			{/each}
		</section>
	{/if}

	<div class="ingestion-grid">
		<section class="ingestion-list">
			<h3>Server review queue</h3>
			{#each ingestions as ingestion}
				<button class:chosen={detail?.ingestion.id === ingestion.id} type="button" on:click={() => void openIngestion(ingestion)}>
					<strong>{ingestion.sourceName}</strong>
					<small>{statusLabel(ingestion.status)} · {ingestion.parsedRowCount} rows</small>
				</button>
			{:else}
				<p class="muted">No parsed statements yet.</p>
			{/each}
		</section>

		{#if detail}
			<section class="ingestion-detail">
				<header class="detail-heading">
					<div>
						<h3>{detail.ingestion.sourceName}</h3>
						<p><span class="status">{statusLabel(detail.ingestion.status)}</span> · {detail.rows.length} parsed rows</p>
						<p>{detail.ingestion.institution || 'Unknown institution'}{#if detail.ingestion.statementDate} · Statement {detail.ingestion.statementDate}{/if}{#if detail.ingestion.paymentDueDate} · Due {detail.ingestion.paymentDueDate}{/if}</p>
					</div>
					<div class="actions">
						{#if detail.ingestion.status === 'ready'}<button type="button" on:click={() => void confirmIngestion()}>Confirm import</button>{/if}
						<button class="danger ghost" type="button" on:click={() => void remove()}>Delete staging</button>
					</div>
				</header>
				{#if validationItems(detail.ingestion.validation).length || detail.ingestion.warnings.length}
					<ul class="validation-list">
						{#each validationItems(detail.ingestion.validation) as item}<li>{item}</li>{/each}
						{#each detail.ingestion.warnings as item}<li>{item.replace(/_/g, ' ')}</li>{/each}
					</ul>
				{/if}
				<p class="muted">Every Save review action is persisted to the server, so review can continue on another device.</p>
				<div class="statement-rows">
					{#each detail.rows as row (row.id)}
						<article class:warning={row.warningCodes.length > 0}>
							<div class="row-main"><input aria-label="Transaction date" type="date" bind:value={row.occurredOn} disabled={detail.ingestion.status === 'confirmed'} /><input aria-label="Merchant" bind:value={row.merchant} disabled={detail.ingestion.status === 'confirmed'} /><input aria-label="Amount" type="number" min="0" step="0.01" value={row.amount / 100} on:change={(event) => changeAmount(row, event)} disabled={detail.ingestion.status === 'confirmed'} /><span>{row.currency}</span></div>
							<div class="row-controls">
								<select aria-label="Statement row kind" bind:value={row.statementKind} disabled={detail.ingestion.status === 'confirmed'}><option value="transaction">Transaction</option><option value="payment">Payment</option><option value="fee">Fee</option><option value="refund">Refund</option><option value="other">Other</option></select>
								<select aria-label="Transaction type" bind:value={row.type} disabled={detail.ingestion.status === 'confirmed'}><option value="expense">Expense</option><option value="income">Income / credit</option></select>
								<select aria-label="Account" bind:value={row.accountId} disabled={detail.ingestion.status === 'confirmed'}>{#each accounts as account}<option value={account.id}>{account.name}</option>{/each}</select>
								<select aria-label="Review decision" value={row.reviewStatus} on:change={(event) => chooseDecision(row, (event.currentTarget as HTMLSelectElement).value)} disabled={detail.ingestion.status === 'confirmed'}>
									<option value="unreviewed">Needs review</option><option value="new">New transaction</option><option value="matched">Match existing</option><option value="ignored">Ignore</option>
								</select>
								<select aria-label="Category" bind:value={row.categoryId} disabled={detail.ingestion.status === 'confirmed'}><option value="">Choose category</option>{#each visibleCategories(row) as category}<option value={category.id}>{category.name}</option>{/each}</select>
								<select aria-label="Matched transaction" value={row.matchExpenseId ?? row.suggestedExpenseId ?? ''} on:change={(event) => chooseMatch(row, (event.currentTarget as HTMLSelectElement).value)} disabled={detail.ingestion.status === 'confirmed'}><option value="">No matched transaction</option>{#each entries as entry}<option value={entry.id}>{entry.id === row.suggestedExpenseId ? 'Suggested · ' : ''}{entry.occurredOn} · {entry.merchant} · {(entry.amount / 100).toFixed(2)}</option>{/each}</select>
								{#if detail.ingestion.status !== 'confirmed'}<button type="button" on:click={() => void saveRow(row)}>Save review</button>{/if}
							</div>
							{#if row.cardholder || row.statementReference}<small class="row-provenance">{row.cardholder || 'Unassigned cardholder'}{#if row.statementReference} · Ref {row.statementReference}{/if}</small>{/if}
							{#if row.suggestedExpenseId && row.reviewStatus === 'unreviewed'}<small class="suggestion-text">Possible match found{#if row.matchConfidence != null} · {Math.round(row.matchConfidence * 100)}% confidence{/if}. Choose “Match existing” to accept it.</small>{/if}
							{#if row.foreignAmount != null && row.foreignCurrency}<small>Original amount: {new Intl.NumberFormat(undefined, { style: 'currency', currency: row.foreignCurrency }).format(row.foreignAmount / 100)}</small>{/if}
							{#if row.warningCodes.length}<small class="warning-text">{row.warningCodes.join(', ').replace(/_/g, ' ')}</small>{/if}
						</article>
					{/each}
				</div>
			</section>
		{/if}
	</div>
</section>

<style>
	.statement-ingestion { display: grid; gap: 1rem; }
	.statement-ingestion header, .detail-heading, .job-row, .row-main, .row-controls, .actions { display: flex; gap: .65rem; align-items: center; }
	.statement-ingestion header, .detail-heading { justify-content: space-between; align-items: flex-start; }
	.statement-ingestion h2, .statement-ingestion h3, .statement-ingestion p { margin: 0; }
	.statement-ingestion p { color: var(--text-muted, #667085); font-size: .88rem; }
	.upload-card, .local-jobs, .ingestion-list, .ingestion-detail { border: 1px solid var(--border, #e4e7ec); border-radius: 12px; padding: 1rem; background: var(--surface, #fff); }
	.upload-card { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr) auto; gap: .75rem; align-items: end; }
	.upload-card label { display: grid; gap: .35rem; font-size: .82rem; font-weight: 700; }
	.local-jobs { display: grid; gap: .6rem; }
	.job-row { justify-content: space-between; border-top: 1px solid var(--border, #e4e7ec); padding-top: .6rem; }
	.job-row div { display: grid; gap: .15rem; }
	.job-row small { color: var(--text-muted, #667085); }
	.ingestion-grid { display: grid; grid-template-columns: minmax(180px, .45fr) minmax(0, 1.55fr); gap: 1rem; }
	.ingestion-list { display: grid; align-content: start; gap: .45rem; }
	.ingestion-list button { display: grid; text-align: left; gap: .18rem; background: transparent; border: 1px solid transparent; padding: .6rem; }
	.ingestion-list button.chosen { border-color: var(--primary, #2563eb); background: #eff6ff; }
	.ingestion-list small { color: var(--text-muted, #667085); }
	.ingestion-detail { min-width: 0; }
	.status { text-transform: capitalize; font-weight: 700; }
	.validation-list { margin: .75rem 0; padding-left: 1.25rem; color: #9a6700; font-size: .85rem; }
	.statement-rows { display: grid; gap: .55rem; max-height: 38rem; overflow: auto; }
	.statement-rows article { border-top: 1px solid var(--border, #e4e7ec); padding-top: .55rem; display: grid; gap: .45rem; }
	.statement-rows article.warning { border-left: 3px solid #f59e0b; padding-left: .45rem; }
	.row-main input:nth-child(2) { flex: 1; min-width: 9rem; }
	.row-main input { min-width: 0; }
	.row-controls { flex-wrap: wrap; }
	.row-controls select { max-width: 14rem; }
	.warning-text, .statement-error { color: #b42318; }
	.row-provenance { color: var(--text-muted, #667085); }
	.suggestion-text { color: #175cd3; }
	.statement-success { color: #027a48 !important; }
	@media (max-width: 780px) { .upload-card, .ingestion-grid { grid-template-columns: 1fr; } .row-main, .row-controls { align-items: stretch; flex-direction: column; } .row-controls select { max-width: none; } }
</style>

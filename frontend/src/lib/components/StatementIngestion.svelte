<script lang="ts">
	import { onMount } from 'svelte';
	import { AlertTriangle, ArrowLeft, CheckCircle2, ChevronRight, FileText, RefreshCw, Search, ShieldCheck, Trash2, Upload } from 'lucide-svelte';
	import { deleteLocalStatementJob, getLocalStatementJobs, putLocalStatementJob } from '$lib/db';
	import { confirmStatementIngestion, createStatementIngestion, deleteStatementIngestion, extractEmbeddedPdfText, getStatementIngestion, listStatementIngestions, parseDbsStatementText, updateStatementRow } from '$lib/statements';
	import type { ParsedStatement } from '$lib/statements';
	import type { Account, Category, LedgerEntry, LocalStatementJob, StatementIngestion, StatementIngestionDetail, StatementIngestionRow, StatementReviewStatus } from '$lib/types';
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
	let detailCache: Record<string, StatementIngestionDetail> = {};
	let loading = false;
	let parsing = false;
	let detailLoading = false;
	let confirming = false;
	let actionError = '';
	let actionMessage = '';
	let fileInput: HTMLInputElement | null = null;
	let dragActive = false;
	let searchQuery = '';
	let statusFilter: 'all' | StatementReviewStatus = 'all';
	let editingRowId = '';
	let rowDraft: StatementIngestionRow | null = null;
	let savingRowId = '';

	$: if (!selectedAccountId && accounts.length) selectedAccountId = accounts[0].id;
	$: filteredRows = (detail?.rows ?? []).filter((row) => {
		const query = searchQuery.trim().toLowerCase();
		return (!query || `${row.merchant} ${row.cardholder} ${row.statementReference}`.toLowerCase().includes(query)) && (statusFilter === 'all' || row.reviewStatus === statusFilter);
	});
	$: reviewCounts = countReviews(detail?.rows ?? []);

	onMount(() => { void refresh(); });

	async function refresh(): Promise<void> {
		if (!groupId) return;
		loading = true; actionError = '';
		try {
			[localJobs, ingestions] = await Promise.all([getLocalStatementJobs(groupId), listStatementIngestions(groupId)]);
			if (detail && !ingestions.some((item) => item.id === detail?.ingestion.id)) closeDetail();
		} catch (error) { actionError = messageFor(error); }
		finally { loading = false; }
	}

	function openFilePicker(): void { fileInput?.click(); }
	function handleFileChange(event: Event): void { selectFile((event.currentTarget as HTMLInputElement).files?.[0] ?? null); }
	function handleDrop(event: DragEvent): void { event.preventDefault(); dragActive = false; selectFile(event.dataTransfer?.files?.[0] ?? null); }
	function selectFile(file: File | null): void {
		actionError = '';
		if (!file) return;
		selectedFile = null;
		if (!file.name.toLowerCase().endsWith('.pdf') && file.type !== 'application/pdf') { actionError = 'Choose a PDF statement.'; return; }
		if (file.size > 25 * 1024 * 1024) { actionError = 'The local PDF limit is 25 MB.'; return; }
		selectedFile = file;
	}

	async function queueFile(): Promise<void> {
		if (!selectedFile || !selectedAccountId || parsing) return;
		const file = selectedFile;
		actionError = ''; actionMessage = ''; parsing = true;
		try {
			const job: LocalStatementJob = { id: makeId(), groupId, accountId: selectedAccountId, file, fileName: file.name, fileType: file.type, sourceFingerprint: await sha256Fingerprint(file), status: 'local_parsing', stage: 'embedded_text', createdAt: isoNow(), updatedAt: isoNow() };
			await putLocalStatementJob(job); localJobs = [job, ...localJobs]; selectedFile = null;
			if (fileInput) fileInput.value = '';
			await resumeLocalJob(job);
		} catch (error) { actionError = messageFor(error); }
		finally { parsing = false; }
	}

	async function resumeLocalJob(job: LocalStatementJob): Promise<void> {
		actionError = ''; actionMessage = '';
		let working: LocalStatementJob = { ...job, sourceFingerprint: job.sourceFingerprint || (await sha256Fingerprint(job.file)), status: 'local_parsing', error: undefined, updatedAt: isoNow() };
		await checkpoint(working);
		try {
			let parsed = working.parsedPayload as unknown as ParsedStatement | undefined;
			if (!parsed) {
				working = { ...working, stage: 'embedded_text', updatedAt: isoNow() }; await checkpoint(working);
				parsed = parseDbsStatementText(await extractEmbeddedPdfText(working.file), working.accountId, working.fileName);
			}
			parsed.clientRequestId = working.id; parsed.sourceFingerprint = working.sourceFingerprint || undefined;
			if (!parsed.rows.length) throw new Error('No statement transactions could be recognized from the local text layer.');
			working = { ...working, stage: 'structured_ready', parsedPayload: parsed as unknown as Record<string, unknown>, updatedAt: isoNow() }; await checkpoint(working);
			working = { ...working, stage: 'saving_structured', updatedAt: isoNow() }; await checkpoint(working);
			const created = await createStatementIngestion(groupId, parsed);
			await deleteLocalStatementJob(job.id); localJobs = localJobs.filter((item) => item.id !== job.id);
			detailCache = { ...detailCache, [created.ingestion.id]: created };
			ingestions = [created.ingestion, ...ingestions.filter((item) => item.id !== created.ingestion.id)];
			actionMessage = `${created.rows.length} normalized rows are ready for review.`;
		} catch (error) {
			const failed: LocalStatementJob = { ...working, status: 'failed', stage: working.stage === 'embedded_text' ? 'ocr' : working.stage, error: messageFor(error), updatedAt: isoNow() };
			await putLocalStatementJob(failed); localJobs = localJobs.map((item) => item.id === failed.id ? failed : item); actionError = failed.error ?? 'Local parsing failed.';
		}
	}

	async function checkpoint(job: LocalStatementJob): Promise<void> { await putLocalStatementJob(job); localJobs = localJobs.map((item) => item.id === job.id ? job : item); }
	async function sha256Fingerprint(file: Blob): Promise<string> {
		if (!globalThis.crypto?.subtle) return '';
		return [...new Uint8Array(await globalThis.crypto.subtle.digest('SHA-256', await file.arrayBuffer()))].map((value) => value.toString(16).padStart(2, '0')).join('');
	}
	async function discardLocalJob(job: LocalStatementJob): Promise<void> {
		if (!window.confirm(`Discard the local PDF checkpoint for ${job.fileName}?`)) return;
		await deleteLocalStatementJob(job.id); localJobs = localJobs.filter((item) => item.id !== job.id);
	}

	async function openIngestion(ingestion: StatementIngestion): Promise<void> {
		actionError = ''; actionMessage = ''; detailLoading = true;
		try { detail = await getStatementIngestion(groupId, ingestion.id); detailCache = { ...detailCache, [ingestion.id]: detail }; searchQuery = ''; statusFilter = 'all'; }
		catch (error) { actionError = messageFor(error); }
		finally { detailLoading = false; }
	}
	function closeDetail(): void { detail = null; editingRowId = ''; rowDraft = null; searchQuery = ''; statusFilter = 'all'; }
	function editRow(row: StatementIngestionRow): void { editingRowId = row.id; rowDraft = { ...row, warningCodes: [...row.warningCodes] }; }
	function cancelEdit(): void { editingRowId = ''; rowDraft = null; }

	async function saveDraft(): Promise<void> {
		if (!detail || !rowDraft || savingRowId) return;
		const draft = rowDraft; actionError = ''; savingRowId = draft.id;
		try {
			const updated = await updateStatementRow(groupId, detail.ingestion.id, draft.id, { occurredOn: draft.occurredOn || '', merchant: draft.merchant, amount: draft.amount, currency: draft.currency, foreignAmount: draft.foreignAmount ?? undefined, foreignCurrency: draft.foreignCurrency, statementKind: draft.statementKind, type: draft.type, accountId: draft.accountId || '', categoryId: draft.categoryId || '', note: draft.note, reviewStatus: draft.reviewStatus, matchExpenseId: draft.matchExpenseId || '', warningCodes: draft.warningCodes });
			detail = updated; detailCache = { ...detailCache, [updated.ingestion.id]: updated }; upsertIngestion(updated.ingestion); cancelEdit(); actionMessage = `Saved ${draft.merchant || draft.sourceRowKey}.`;
		} catch (error) { actionError = messageFor(error); }
		finally { savingRowId = ''; }
	}
	function chooseMatch(value: string): void { if (!rowDraft) return; rowDraft.matchExpenseId = value || null; if (value) rowDraft.reviewStatus = 'matched'; rowDraft = { ...rowDraft }; }
	function chooseDecision(value: string): void {
		if (!rowDraft) return; rowDraft.reviewStatus = value as StatementReviewStatus;
		if (rowDraft.reviewStatus === 'matched' && !rowDraft.matchExpenseId && rowDraft.suggestedExpenseId) rowDraft.matchExpenseId = rowDraft.suggestedExpenseId;
		else if (rowDraft.reviewStatus !== 'matched') rowDraft.matchExpenseId = null;
		rowDraft = { ...rowDraft };
	}
	function changeAmount(event: Event): void { if (!rowDraft) return; const value = Number((event.currentTarget as HTMLInputElement).value); rowDraft.amount = Number.isFinite(value) ? Math.max(0, Math.round(value * 100)) : 0; rowDraft = { ...rowDraft }; }

	async function confirmIngestion(): Promise<void> {
		if (!detail || confirming || !window.confirm('Confirm all reviewed rows? New rows will be inserted and matched rows will update the selected transactions.')) return;
		confirming = true; actionError = '';
		try { detail = await confirmStatementIngestion(groupId, detail.ingestion.id); detailCache = { ...detailCache, [detail.ingestion.id]: detail }; upsertIngestion(detail.ingestion); await onConfirmed?.(); actionMessage = 'Statement ingestion confirmed.'; }
		catch (error) { actionError = messageFor(error); }
		finally { confirming = false; }
	}
	async function remove(): Promise<void> {
		if (!detail || !window.confirm('Soft-delete this ingestion and its staging rows? Confirmed transactions will be retained.')) return;
		try { const id = detail.ingestion.id; await deleteStatementIngestion(groupId, id); ingestions = ingestions.filter((item) => item.id !== id); closeDetail(); actionMessage = 'Statement staging data deleted; confirmed transactions were kept.'; }
		catch (error) { actionError = messageFor(error); }
	}

	function upsertIngestion(next: StatementIngestion): void { ingestions = [next, ...ingestions.filter((item) => item.id !== next.id)]; }
	function accountFor(id: string): Account | undefined { return accounts.find((account) => account.id === id); }
	function visibleCategories(row: StatementIngestionRow): Category[] { return categories.filter((category) => category.type === row.type && !category.deletedAt); }
	function countReviews(rows: StatementIngestionRow[]): Record<StatementReviewStatus, number> { const counts = { unreviewed: 0, new: 0, matched: 0, ignored: 0 }; for (const row of rows) counts[row.reviewStatus]++; return counts; }
	function cachedSummary(ingestion: StatementIngestion): string {
		const cached = detailCache[ingestion.id]; if (!cached) return statusInfo(ingestion.status).label;
		const counts = countReviews(cached.rows); return ingestion.status === 'confirmed' ? `${counts.new} created · ${counts.matched} updated` : `${counts.matched} matched · ${counts.new} new${counts.unreviewed ? ` · ${counts.unreviewed} to review` : ''}`;
	}
	function statusInfo(status: StatementIngestion['status']): { label: string; tone: string } {
		if (status === 'confirmed') return { label: 'Confirmed', tone: 'confirmed' }; if (status === 'ready') return { label: 'Ready to confirm', tone: 'ready' }; if (status === 'failed') return { label: 'Needs attention', tone: 'danger' }; if (status === 'parsed') return { label: 'Parsed', tone: 'processing' }; return { label: 'Review required', tone: 'review' };
	}
	function validationItems(validation: Record<string, unknown>, currency = 'SGD', warnings: string[] = []): Array<{ label: string; pass: boolean }> {
		const output: Array<{ label: string; pass: boolean }> = [];
		const newTotal = validation.newTransactionsTotal as { pass?: boolean; actual?: number } | undefined;
		const balance = validation.balanceReconciliation as { pass?: boolean; expectedGrandTotal?: number } | undefined;
		const unreadable = validation.unreadableRows as unknown[] | undefined;
		const foreign = validation.foreignCurrencyRows as unknown[] | undefined;
		const cardholders = validation.cardholders as Array<{ name?: string; actualRowCount?: number; actualSubtotal?: number; rowCountPass?: boolean; subtotalPass?: boolean }> | undefined;
		if (newTotal) output.push({ label: `New transactions ${money(newTotal.actual ?? 0, currency)}`, pass: Boolean(newTotal.pass) });
		if (balance) output.push({ label: `Statement total ${money(balance.expectedGrandTotal ?? 0, currency)}`, pass: Boolean(balance.pass) });
		output.push({ label: unreadable?.length ? `${unreadable.length} unreadable row${unreadable.length === 1 ? '' : 's'}` : 'All rows readable', pass: !unreadable?.length });
		if (foreign?.length) output.push({ label: `${foreign.length} foreign-currency row${foreign.length === 1 ? '' : 's'}`, pass: true });
		for (const cardholder of cardholders ?? []) output.push({ label: `${cardholder.name || 'Cardholder'} · ${cardholder.actualRowCount ?? 0} rows · ${money(cardholder.actualSubtotal ?? 0, currency)}`, pass: cardholder.rowCountPass !== false && cardholder.subtotalPass !== false });
		const representedWarnings = new Set<string>();
		if (unreadable?.length) representedWarnings.add('unreadable_rows');
		if (foreign?.length) representedWarnings.add('foreign_currency_rows');
		for (const warning of warnings) if (!representedWarnings.has(warning)) output.push({ label: warning.replace(/_/g, ' '), pass: false });
		return output;
	}
	function reviewLabel(status: StatementReviewStatus): string { return ({ unreviewed: 'Needs review', new: 'New', matched: 'Matched', ignored: 'Ignored' })[status]; }
	function localStage(job: LocalStatementJob): string { if (job.status === 'failed') return 'Needs attention'; return ({ embedded_text: 'Extracting locally', ocr: 'Reading locally', structured_ready: 'Preparing rows', saving_structured: 'Saving normalized rows', failed: 'Needs attention' })[job.stage]; }
	function institutionMark(name: string): string { return (name.trim().split(/\s+/)[0] || 'PDF').slice(0, 4).toUpperCase(); }
	function periodLabel(start?: string | null, end?: string | null): string { if (!start && !end) return 'Statement period unavailable'; return `${shortDate(start)}${start !== end && end ? ` – ${shortDate(end)}` : ''}`; }
	function shortDate(value?: string | null): string { if (!value) return '—'; return new Intl.DateTimeFormat(undefined, { day: 'numeric', month: 'short', year: 'numeric' }).format(new Date(`${value}T00:00:00`)); }
	function money(cents: number, currency = detail?.ingestion.statementCurrency ?? 'SGD'): string { return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(cents / 100); }
	function messageFor(error: unknown): string { return error instanceof Error ? error.message : 'Unexpected statement ingestion error.'; }
</script>

<section class="statement-ingestion" aria-label="Statement ingestion">
	{#if !detail}
		<header class="page-heading">
			<div><h2>Statement ingestion</h2><p>Parse, review, and reconcile bank statement transactions.</p></div>
			<button class="primary-action" type="button" on:click={selectedFile ? () => void queueFile() : openFilePicker} disabled={parsing || !accounts.length}><Upload size={18} />{selectedFile ? (parsing ? 'Parsing locally…' : 'Parse statement') : 'Choose statement'}</button>
		</header>
		<input class="hidden-file" bind:this={fileInput} type="file" accept="application/pdf,.pdf" on:change={handleFileChange} />
		<div class:drag-active={dragActive} class="drop-zone" role="button" tabindex="0" on:click={openFilePicker} on:keydown={(event) => (event.key === 'Enter' || event.key === ' ') && openFilePicker()} on:dragenter|preventDefault={() => (dragActive = true)} on:dragover|preventDefault={() => (dragActive = true)} on:dragleave|preventDefault={() => (dragActive = false)} on:drop={handleDrop}>
			<span class="drop-icon"><FileText size={25} /></span>
			<div class="drop-copy"><strong>{selectedFile?.name ?? 'Drop a PDF statement here'}</strong><span>{selectedFile ? 'Ready to parse entirely on this device' : 'PDF up to 25 MB · The original file never leaves this device'}</span></div>
			<button class="secondary-action" type="button" on:click|stopPropagation={openFilePicker}>Choose PDF</button>
		</div>
		<div class="upload-options">
			<label><span>Import into</span><select bind:value={selectedAccountId} aria-label="Card account">{#each accounts as account}<option value={account.id}>{account.name}</option>{/each}</select></label>
			<div class="privacy-note"><ShieldCheck size={16} /><span>Only normalized transaction fields are saved after local parsing.</span></div>
			{#if selectedFile}<button class="inline-primary" type="button" on:click={() => void queueFile()} disabled={parsing || !selectedAccountId}>{parsing ? 'Parsing…' : 'Parse locally'}</button>{/if}
		</div>
		{#if actionError}<p class="feedback error" role="alert"><AlertTriangle size={17} />{actionError}</p>{/if}
		{#if actionMessage}<p class="feedback success"><CheckCircle2 size={17} />{actionMessage}</p>{/if}

		<section class="recent-card">
			<header class="recent-heading"><div><h3>Recent statements</h3><p>{localJobs.length + ingestions.length} statement{localJobs.length + ingestions.length === 1 ? '' : 's'}</p></div><button class="icon-button" type="button" aria-label="Refresh statements" on:click={() => void refresh()} disabled={loading}><RefreshCw size={18} class={loading ? 'spinning' : ''} /></button></header>
			<div class="statement-list">
				{#each localJobs as job (job.id)}
					<article class="statement-item local-item"><span class={`bank-mark ${job.status === 'failed' ? 'danger' : 'processing'}`}>PDF</span><div class="statement-name"><strong>{accountFor(job.accountId)?.name ?? job.fileName}</strong><small>{job.fileName}</small></div><div class="statement-summary"><strong>{job.status === 'failed' ? 'Parsing failed' : 'Processing'}</strong><small>{job.error ?? localStage(job)}</small></div><div class="statement-period"><strong>On-device checkpoint</strong><small>Safe to close and resume</small></div><span class={`status-pill ${job.status === 'failed' ? 'danger' : 'processing'}`}><i></i>{localStage(job)}</span><div class="local-actions"><button type="button" on:click={() => void resumeLocalJob(job)}>Resume</button><button type="button" aria-label={`Discard ${job.fileName}`} on:click={() => void discardLocalJob(job)}><Trash2 size={16} /></button></div></article>
				{/each}
				{#each ingestions as ingestion (ingestion.id)}
					{@const status = statusInfo(ingestion.status)}
					<button class="statement-item" type="button" on:click={() => void openIngestion(ingestion)}><span class={`bank-mark ${status.tone}`}>{institutionMark(ingestion.institution)}</span><span class="statement-name"><strong>{accountFor(ingestion.accountId)?.name ?? ingestion.institution ?? 'Statement'}</strong><small>{ingestion.sourceName}</small></span><span class="statement-summary"><strong>{ingestion.parsedRowCount} transactions</strong><small>{cachedSummary(ingestion)}</small></span><span class="statement-period"><strong>{periodLabel(ingestion.periodStart, ingestion.periodEnd)}</strong><small>{ingestion.statementDate ? `Statement ${shortDate(ingestion.statementDate)}` : 'Statement date unavailable'}</small></span><span class={`status-pill ${status.tone}`}><i></i>{status.label}</span><span class="chevron"><ChevronRight size={20} /></span></button>
				{:else}{#if !localJobs.length}<div class="empty-state"><FileText size={28} /><strong>No statements yet</strong><span>Choose a PDF to start a private, on-device import.</span></div>{/if}{/each}
			</div>
		</section>
	{:else}
		<header class="detail-heading">
			<div class="detail-title"><button class="back-button" type="button" aria-label="Back to statements" on:click={closeDetail}><ArrowLeft size={19} /></button><div><p class="eyebrow">{accountFor(detail.ingestion.accountId)?.name ?? detail.ingestion.institution}</p><h2>{detail.ingestion.sourceName}</h2><p>{periodLabel(detail.ingestion.periodStart, detail.ingestion.periodEnd)} · {detail.rows.length} parsed transactions</p></div></div>
			<div class="detail-actions"><button class="danger-action" type="button" on:click={() => void remove()}><Trash2 size={16} /> Delete staging</button>{#if detail.ingestion.status === 'ready'}<button class="primary-action" type="button" on:click={() => void confirmIngestion()} disabled={confirming}><CheckCircle2 size={18} />{confirming ? 'Confirming…' : 'Confirm import'}</button>{/if}</div>
		</header>
		{#if actionError}<p class="feedback error" role="alert"><AlertTriangle size={17} />{actionError}</p>{/if}
		{#if actionMessage}<p class="feedback success"><CheckCircle2 size={17} />{actionMessage}</p>{/if}
		<div class="summary-grid"><article><span>Review progress</span><strong>{detail.rows.length - reviewCounts.unreviewed} / {detail.rows.length}</strong><small>{reviewCounts.unreviewed ? `${reviewCounts.unreviewed} still need review` : 'All rows reviewed'}</small></article><article><span>Matched</span><strong>{reviewCounts.matched}</strong><small>Existing transactions to update</small></article><article><span>New</span><strong>{reviewCounts.new}</strong><small>Transactions to create</small></article><article><span>Statement total</span><strong>{money(detail.ingestion.statementGrandTotal ?? detail.ingestion.declaredNewTransactionsTotal ?? 0)}</strong><small>{detail.ingestion.paymentDueDate ? `Due ${shortDate(detail.ingestion.paymentDueDate)}` : 'Reconciliation amount'}</small></article></div>
		<section class="checks-card"><div class="checks-heading"><ShieldCheck size={19} /><div><strong>Reconciliation checks</strong><span>Amounts use the posted statement currency.</span></div></div><div class="checks-list">{#each validationItems(detail.ingestion.validation, detail.ingestion.statementCurrency, detail.ingestion.warnings) as check}<span class:check-warning={!check.pass}>{#if check.pass}<CheckCircle2 size={16} />{:else}<AlertTriangle size={16} />{/if}{check.label}</span>{/each}</div></section>
		<section class="transactions-card">
			<header class="transactions-toolbar"><div><h3>Transactions</h3><p>Each saved decision is durable and can be continued later.</p></div><div class="toolbar-controls"><label class="search-control"><Search size={17} /><input bind:value={searchQuery} type="search" placeholder="Search merchant or reference" aria-label="Search statement transactions" /></label><select bind:value={statusFilter} aria-label="Filter review status"><option value="all">All statuses</option><option value="unreviewed">Needs review</option><option value="matched">Matched</option><option value="new">New</option><option value="ignored">Ignored</option></select></div></header>
			<div class="transaction-head" aria-hidden="true"><span>Date</span><span>Merchant</span><span>Amount</span><span>Category</span><span>Status</span><span></span></div>
			<div class="transaction-list">
				{#each filteredRows as row (row.id)}
					<article class:row-warning={row.warningCodes.length > 0} class:editing={editingRowId === row.id} class="transaction-row">
						<button class="row-summary" type="button" on:click={() => editingRowId === row.id ? cancelEdit() : editRow(row)} disabled={detail.ingestion.status === 'confirmed'}><span class="date-cell" data-label="Date">{shortDate(row.occurredOn)}</span><span class="merchant-cell" data-label="Merchant"><strong>{row.merchant || 'Unreadable transaction'}</strong><small>{row.cardholder}{row.statementReference ? ` · ${row.statementReference}` : ''}</small></span><span class="amount-cell" data-label="Amount"><strong class:credit={row.type === 'income'}>{row.type === 'income' ? '−' : ''}{money(row.amount, row.currency)}</strong>{#if row.foreignAmount != null && row.foreignCurrency}<small>{money(row.foreignAmount, row.foreignCurrency)} original</small>{/if}</span><span class="category-cell" data-label="Category">{categories.find((category) => category.id === row.categoryId)?.name ?? 'Uncategorised'}</span><span class={`row-status ${row.reviewStatus}`}>{reviewLabel(row.reviewStatus)}</span><span class="row-chevron"><ChevronRight size={18} /></span></button>
						{#if editingRowId === row.id && rowDraft}
							<div class="row-editor"><div class="editor-grid"><label>Date<input aria-label="Transaction date" type="date" bind:value={rowDraft.occurredOn} /></label><label class="merchant-field">Merchant<input aria-label="Merchant" bind:value={rowDraft.merchant} /></label><label>Amount<input aria-label="Amount" type="number" min="0" step="0.01" value={rowDraft.amount / 100} on:change={changeAmount} /></label><label>Kind<select aria-label="Statement row kind" bind:value={rowDraft.statementKind}><option value="transaction">Transaction</option><option value="payment">Payment</option><option value="fee">Fee</option><option value="refund">Refund</option><option value="other">Other</option></select></label><label>Type<select aria-label="Transaction type" bind:value={rowDraft.type}><option value="expense">Expense</option><option value="income">Income / credit</option></select></label><label>Account<select aria-label="Account" bind:value={rowDraft.accountId}>{#each accounts as account}<option value={account.id}>{account.name}</option>{/each}</select></label><label>Category<select aria-label="Category" bind:value={rowDraft.categoryId}><option value="">Choose category</option>{#each visibleCategories(rowDraft) as category}<option value={category.id}>{category.name}</option>{/each}</select></label><label>Decision<select aria-label="Review decision" value={rowDraft.reviewStatus} on:change={(event) => chooseDecision((event.currentTarget as HTMLSelectElement).value)}><option value="unreviewed">Needs review</option><option value="new">Create new</option><option value="matched">Match existing</option><option value="ignored">Ignore</option></select></label>{#if rowDraft.reviewStatus === 'matched'}<label class="match-field">Matched transaction<select aria-label="Matched transaction" value={rowDraft.matchExpenseId ?? rowDraft.suggestedExpenseId ?? ''} on:change={(event) => chooseMatch((event.currentTarget as HTMLSelectElement).value)}><option value="">Choose transaction</option>{#each entries as entry}<option value={entry.id}>{entry.id === rowDraft.suggestedExpenseId ? 'Suggested · ' : ''}{entry.occurredOn} · {entry.merchant} · {money(entry.amount, entry.currency)}</option>{/each}</select></label>{/if}</div><div class="editor-footer"><div>{#if rowDraft.suggestedExpenseId}<span class="suggestion">Suggested match{rowDraft.matchConfidence != null ? ` · ${Math.round(rowDraft.matchConfidence * 100)}% confidence` : ''}</span>{/if}{#if rowDraft.warningCodes.length}<span class="warning-text">{rowDraft.warningCodes.join(', ').replace(/_/g, ' ')}</span>{/if}</div><div class="editor-actions"><button type="button" on:click={cancelEdit}>Cancel</button><button class="inline-primary" type="button" on:click={() => void saveDraft()} disabled={savingRowId === rowDraft.id}>{savingRowId === rowDraft.id ? 'Saving…' : 'Save decision'}</button></div></div></div>
						{/if}
					</article>
				{:else}<div class="empty-state"><Search size={24} /><strong>No matching transactions</strong><span>Try another search or status filter.</span></div>{/each}
			</div>
		</section>
	{/if}
	{#if detailLoading}<div class="loading-overlay"><RefreshCw class="spinning" size={22} /><span>Loading statement…</span></div>{/if}
</section>

<style>
	.statement-ingestion{color:var(--ink);display:grid;gap:1.5rem;min-width:0;position:relative}.statement-ingestion button{width:auto}.statement-ingestion h2,.statement-ingestion h3,.statement-ingestion p{margin:0}.statement-ingestion h2{font-size:clamp(1.8rem,3vw,2.5rem);letter-spacing:-.035em;line-height:1.05}.statement-ingestion h3{font-size:1.1rem}.statement-ingestion p,.statement-ingestion small,.drop-copy span{color:var(--muted)}
	.page-heading,.detail-heading,.recent-heading,.transactions-toolbar,.editor-footer{align-items:center;display:flex;gap:1rem;justify-content:space-between}.page-heading>div,.detail-title>div{display:grid;gap:.45rem}.primary-action,.inline-primary{background:var(--primary);border:1px solid var(--primary);border-radius:12px;color:#0a1630;font-weight:850;gap:.55rem;min-height:2.9rem;padding:.75rem 1.1rem}.secondary-action,.danger-action,.back-button,.icon-button,.editor-actions button,.local-actions button{background:transparent;border:1px solid var(--line);border-radius:11px;color:var(--ink);font-weight:750;min-height:2.6rem;padding:.6rem .9rem}.danger-action{color:#ef8f87;gap:.5rem}.hidden-file{display:none}
	.drop-zone{align-items:center;background:color-mix(in srgb,var(--primary) 5%,var(--panel));border:1.5px dashed var(--primary);border-radius:18px;cursor:pointer;display:grid;gap:1.1rem;grid-template-columns:auto 1fr auto;min-height:9.5rem;padding:1.7rem 2rem;transition:.15s}.drop-zone:hover,.drop-zone.drag-active{background:color-mix(in srgb,var(--primary) 11%,var(--panel));transform:translateY(-1px)}.drop-zone:focus-visible{outline:3px solid color-mix(in srgb,var(--primary) 55%,transparent);outline-offset:3px}.drop-icon{align-items:center;background:color-mix(in srgb,var(--primary) 24%,transparent);border-radius:14px;color:var(--primary);display:flex;height:4.3rem;justify-content:center;width:4.3rem}.drop-copy{display:grid;gap:.35rem}.drop-copy strong{font-size:1.25rem}.upload-options{align-items:center;display:flex;gap:1rem;margin-top:-1rem}.upload-options label{align-items:center;display:flex;flex:0 1 25rem;gap:.65rem}.upload-options label span{flex:none;font-size:.78rem;letter-spacing:.07em;text-transform:uppercase}.upload-options select,.toolbar-controls select{background:var(--field);border:1px solid var(--line);border-radius:10px;color:var(--ink);min-height:2.5rem;padding:.5rem .75rem}.upload-options select{width:100%}.privacy-note{align-items:center;color:var(--muted);display:flex;flex:1;font-size:.82rem;gap:.45rem}
	.feedback{align-items:center;border:1px solid;border-radius:11px;display:flex;font-size:.9rem;gap:.55rem;padding:.7rem .85rem}.feedback.error{background:#ef5e5317;border-color:#ef5e5359;color:#ef8f87}.feedback.success{background:#3dccaa17;border-color:#3dccaa4d;color:#3dccaa}.recent-card,.transactions-card,.checks-card{background:var(--panel);border:1px solid var(--line);border-radius:18px;overflow:hidden}.recent-heading,.transactions-toolbar{padding:1.4rem 1.6rem}.recent-heading>div,.transactions-toolbar>div:first-child{display:grid;gap:.3rem}.recent-heading p,.transactions-toolbar p{font-size:.85rem}.icon-button,.back-button{align-items:center;display:inline-flex;justify-content:center;padding:0;width:2.7rem!important}.statement-list{border-top:1px solid var(--line)}
	.statement-item{align-items:center;background:transparent;border-bottom:1px solid var(--line);color:var(--ink);display:grid;gap:1.1rem;grid-template-columns:auto minmax(10rem,1.25fr) minmax(9rem,.8fr) minmax(10rem,.9fr) auto auto;min-height:7rem;padding:1rem 1.6rem;text-align:left;width:100%!important}button.statement-item:hover{background:var(--panel-hover)}.statement-item:last-child{border-bottom:0}.bank-mark{align-items:center;background:#d04c523f;border-radius:13px;color:#f29aa0;display:flex;font-size:.85rem;font-weight:850;height:3.5rem;justify-content:center;width:3.5rem}.bank-mark.processing{background:#6794ff33;color:var(--primary)}.bank-mark.confirmed{background:#3dccaa2e;color:#3dccaa}.statement-name,.statement-summary,.statement-period{display:grid;gap:.32rem;min-width:0}.statement-name strong,.statement-name small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.statement-name small,.statement-summary small,.statement-period small{font-size:.82rem}.status-pill,.row-status{align-items:center;border-radius:999px;display:inline-flex;font-size:.82rem;font-weight:800;gap:.42rem;justify-content:center;padding:.55rem .8rem;white-space:nowrap}.status-pill i{background:currentColor;border-radius:50%;height:.48rem;width:.48rem}.status-pill.review,.row-status.unreviewed{background:#e2a1332e;color:#edbb60}.status-pill.processing{background:#6794ff2b;color:var(--primary)}.status-pill.confirmed,.row-status.matched{background:#3dccaa29;color:#3dccaa}.status-pill.ready,.row-status.new{background:#a287e72b;color:#bea9f3}.status-pill.danger{background:#ef5e5326;color:#ef8f87}.row-status.ignored{background:color-mix(in srgb,var(--muted) 12%,transparent);color:var(--muted)}.chevron{color:var(--muted)}.local-actions{display:flex;gap:.45rem}.local-actions button{padding:.45rem .65rem}.empty-state{align-items:center;color:var(--muted);display:flex;flex-direction:column;gap:.4rem;justify-content:center;min-height:11rem;padding:2rem;text-align:center}.empty-state strong{color:var(--ink)}
	.detail-title{align-items:center;display:flex;gap:1rem;min-width:0}.detail-title h2{font-size:clamp(1.35rem,2.4vw,2rem);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.eyebrow{color:var(--primary)!important;font-size:.76rem;font-weight:850;letter-spacing:.08em;text-transform:uppercase}.detail-actions,.editor-actions{display:flex;flex-wrap:wrap;gap:.65rem}.summary-grid{display:grid;gap:.8rem;grid-template-columns:repeat(4,minmax(0,1fr))}.summary-grid article{background:var(--panel);border:1px solid var(--line);border-radius:15px;display:grid;gap:.3rem;padding:1rem 1.1rem}.summary-grid span,.summary-grid small{color:var(--muted);font-size:.78rem}.summary-grid strong{font-size:1.3rem}.checks-card{align-items:center;display:flex;gap:1rem;justify-content:space-between;padding:1rem 1.2rem}.checks-heading{align-items:center;display:flex;gap:.65rem}.checks-heading>div{display:grid;gap:.2rem}.checks-heading span{color:var(--muted);font-size:.78rem}.checks-list{display:flex;flex-wrap:wrap;gap:.55rem;justify-content:flex-end}.checks-list>span{align-items:center;background:#3dccaa17;border-radius:999px;color:#3dccaa;display:flex;font-size:.78rem;font-weight:750;gap:.35rem;padding:.5rem .65rem}.checks-list>span.check-warning{background:#e2a1331c;color:#edbb60}
	.transactions-toolbar{border-bottom:1px solid var(--line)}.toolbar-controls{align-items:center;display:flex;gap:.65rem}.search-control{align-items:center;background:var(--field);border:1px solid var(--line);border-radius:11px;color:var(--muted);display:flex;gap:.45rem;padding:0 .75rem}.search-control input{background:transparent;border:0;color:var(--ink);min-height:2.65rem;outline:0;padding:.55rem 0;width:min(21rem,32vw)}.toolbar-controls select{width:auto}.transaction-head,.row-summary{align-items:center;display:grid;gap:.8rem;grid-template-columns:7.4rem minmax(13rem,1.4fr) minmax(7.5rem,.6fr) minmax(8rem,.7fr) 7.4rem 1.2rem}.transaction-head{background:color-mix(in srgb,var(--panel) 90%,var(--paper));color:var(--muted);font-size:.72rem;font-weight:850;letter-spacing:.06em;padding:.65rem 1.2rem;text-transform:uppercase}.transaction-list{display:block;gap:0}.transaction-row{align-items:stretch;background:transparent;border:0;border-radius:0;border-top:1px solid var(--line);box-shadow:none;display:block;gap:0;grid-template-columns:none;padding:0}.transaction-row strong{color:inherit;font-size:inherit}.transaction-head+.transaction-list .transaction-row:first-child{border-top:0}.row-summary{background:transparent;color:var(--ink);min-height:4.6rem;padding:.75rem 1.2rem;text-align:left;width:100%!important}.row-summary:hover{background:var(--panel-hover)}.transaction-row.editing{background:color-mix(in srgb,var(--primary) 4%,var(--panel))}.transaction-row.row-warning{box-shadow:inset 3px 0 #edbb60}.merchant-cell,.amount-cell{display:grid;gap:.2rem;min-width:0}.merchant-cell strong,.merchant-cell small{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.merchant-cell small,.amount-cell small{font-size:.74rem}.amount-cell strong.credit{color:#3dccaa}.category-cell{color:var(--muted);overflow:hidden;text-overflow:ellipsis;white-space:nowrap}.row-chevron{color:var(--muted);transition:.15s}.transaction-row.editing .row-chevron{transform:rotate(90deg)}
	.row-editor{border-top:1px solid var(--line);padding:1rem 1.2rem 1.15rem}.editor-grid{display:grid;gap:.8rem;grid-template-columns:repeat(4,minmax(0,1fr))}.editor-grid label{color:var(--muted);display:grid;font-size:.75rem;font-weight:800;gap:.4rem}.editor-grid input,.editor-grid select{background:var(--field);border:1px solid var(--line);border-radius:10px;color:var(--ink);min-height:2.75rem;outline:0;padding:.6rem .75rem;width:100%}.editor-grid .merchant-field,.editor-grid .match-field{grid-column:span 2}.editor-footer{border-top:1px solid var(--line);margin-top:1rem;padding-top:1rem}.editor-footer>div:first-child{display:flex;flex-wrap:wrap;gap:.5rem}.suggestion,.warning-text{border-radius:999px;font-size:.76rem;font-weight:750;padding:.35rem .55rem}.suggestion{background:#6794ff21;color:var(--primary)}.warning-text{background:#e2a1331f;color:#edbb60}.editor-actions button{min-height:2.5rem}.loading-overlay{align-items:center;backdrop-filter:blur(3px);background:color-mix(in srgb,var(--paper) 80%,transparent);border-radius:18px;display:flex;gap:.6rem;inset:0;justify-content:center;position:absolute;z-index:10}
	@media(max-width:1050px){.statement-item{grid-template-columns:auto minmax(10rem,1fr) minmax(8rem,.7fr) auto auto}.statement-period{display:none}.summary-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.checks-card{align-items:flex-start;flex-direction:column}.checks-list{justify-content:flex-start}.editor-grid{grid-template-columns:repeat(2,minmax(0,1fr))}}
	@media(max-width:767px){.statement-ingestion{gap:1rem}.page-heading,.detail-heading,.transactions-toolbar,.editor-footer{align-items:stretch;flex-direction:column}.page-heading .primary-action{align-self:stretch}.drop-zone{grid-template-columns:auto 1fr;min-height:8rem;padding:1.15rem}.drop-zone .secondary-action{grid-column:1/-1;width:100%}.drop-icon{height:3.4rem;width:3.4rem}.drop-copy strong{font-size:1rem}.upload-options{align-items:stretch;flex-direction:column;margin-top:-.35rem}.upload-options label{align-items:stretch;display:grid;flex-basis:auto}.privacy-note{align-items:flex-start}.inline-primary{width:100%!important}.recent-heading{padding:1rem}.statement-item,.statement-item.local-item{gap:.8rem;grid-template-columns:auto 1fr auto;min-height:auto;padding:1rem}.statement-name{grid-column:2}.statement-summary{grid-column:2}.statement-period{display:none}.status-pill{grid-column:2;justify-self:start}.chevron{grid-column:3;grid-row:1/4}.local-actions{grid-column:3;grid-row:1/4;flex-direction:column}.detail-title{align-items:flex-start}.detail-title h2{white-space:normal;word-break:break-word}.detail-actions{display:grid}.detail-actions button{width:100%}.summary-grid{grid-template-columns:repeat(2,minmax(0,1fr))}.summary-grid article{padding:.85rem}.checks-card{border-radius:14px}.checks-list{display:grid;width:100%}.toolbar-controls{align-items:stretch;flex-direction:column}.search-control input{width:100%}.toolbar-controls select{width:100%}.transaction-head{display:none}.row-summary{gap:.55rem;grid-template-columns:1fr auto;padding:.9rem 1rem}.row-summary>span::before{color:var(--muted);content:attr(data-label);display:block;font-size:.65rem;font-weight:800;letter-spacing:.06em;margin-bottom:.18rem;text-transform:uppercase}.date-cell{grid-column:1;grid-row:1}.amount-cell{grid-column:2;grid-row:1;text-align:right}.merchant-cell{grid-column:1/-1;grid-row:2}.category-cell{grid-column:1;grid-row:3}.row-status{grid-column:2;grid-row:3;justify-self:end}.row-chevron{display:none}.row-editor{padding:1rem}.editor-grid{grid-template-columns:1fr}.editor-grid .merchant-field,.editor-grid .match-field{grid-column:auto}.editor-actions{display:grid;grid-template-columns:1fr 1fr}.editor-actions button{width:100%}}
	.bank-mark.danger{background:#ef5e5326;color:#ef8f87}
</style>

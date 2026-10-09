import assert from 'node:assert/strict';
import { createServer } from 'vite';

const server = await createServer({
	configFile: false, server: { middlewareMode: true }, appType: 'custom', logLevel: 'silent',
	plugins: [{
		name: 'sync-test-storage',
		enforce: 'pre',
		resolveId(source, importer) {
			if (importer?.split('?')[0].endsWith('/src/lib/sync.ts')) {
				if (source === './db' || source.endsWith('/lib/db.ts')) return '\0sync-test:./db';
				if (source === './auth' || source.endsWith('/lib/auth.ts')) return '\0sync-test:./auth';
			}
		},
		load(id) {
			if (id === '\0sync-test:./db') return `
				export const getSyncCheckpoint = async (id) => structuredClone(globalThis.__syncTest.checkpoints.get(id) ?? { id, fingerprints: {} });
				export const saveSyncCheckpoint = async (checkpoint) => globalThis.__syncTest.checkpoints.set(checkpoint.id, structuredClone(checkpoint));
				export const mergeRemoteState = async (remote) => {
					const merged = globalThis.__syncTest.merged;
					for (const key of Object.keys(remote)) for (const row of remote[key]) {
						const index = merged[key].findIndex(item => item.id === row.id);
						if (index < 0) merged[key].push(row); else merged[key][index] = row;
					}
				};
			`;
			if (id === '\0sync-test:./auth') return `
				export const getStoredSession = () => ({ user: { id: globalThis.__syncTest.userId ?? 'test-user' } });
				export const authFetch = (...args) => globalThis.__syncTest.fetch(...args);
			`;
		}
	}]
});
const originalFetch = globalThis.fetch;
const originalConsoleError = console.error;
const logs = [];

try {
	const { readSyncError } = await server.ssrLoadModule('/src/lib/sync-error.ts');
	const { proxyApiRequest } = await server.ssrLoadModule('/src/lib/server/api-proxy.ts');
	console.error = (...args) => logs.push(args);

	const request = () => new Request('https://expenses.test/api/v1/sync?private=query-secret', {
		method: 'POST', headers: { authorization: 'Bearer token-secret', 'content-type': 'application/json' },
		body: JSON.stringify({ entries: [{ note: 'financial-secret' }] })
	});
	let forwardedId;
	globalThis.fetch = async (target, init) => {
		assert.equal(target, 'http://backend:8080/api/v1/sync?private=query-secret');
		assert.equal(init.headers.get('authorization'), 'Bearer token-secret');
		forwardedId = init.headers.get('x-request-id');
		assert.match(forwardedId, /^[a-f0-9-]{36}$/);
		assert.match(new TextDecoder().decode(init.body), /financial-secret/);
		return Response.json({ success: false, error: 'upsert entry: foreign key constraint' }, {
			status: 500, headers: { 'x-request-id': forwardedId }
		});
	};
	const failed = await proxyApiRequest(request(), 'sync', 'http://backend:8080');
	assert.equal(failed.status, 500);
	assert.equal(failed.headers.get('cache-control'), 'no-store');
	const error = await readSyncError(failed);
	assert.match(error.message, /HTTP 500/);
	assert.match(error.message, /foreign key constraint/);
	assert.ok(error.message.includes(forwardedId));
	assert.equal(logs[0][1].requestId, forwardedId);
	assert.equal(logs[0][1].status, 500);

	globalThis.fetch = async () => { throw new TypeError('fetch failed', { cause: { code: 'ECONNREFUSED' } }); };
	const unreachable = await proxyApiRequest(request(), 'sync', 'http://backend:8080');
	assert.equal(unreachable.status, 502);
	const connectionError = await readSyncError(unreachable);
	assert.match(connectionError.message, /could not reach the backend/);
	assert.ok(connectionError.message.includes(unreachable.headers.get('x-request-id')));
	assert.equal(logs[1][1].code, 'ECONNREFUSED');

	const htmlError = await readSyncError(new Response('<html>Internal Server Error</html>', {
		status: 500, headers: { 'x-request-id': 'gateway-request' }
	}));
	assert.match(htmlError.message, /HTTP 500/);
	assert.match(htmlError.message, /no error details/);
	assert.match(htmlError.message, /gateway-request/);
	assert.ok(!htmlError.message.includes('<html>'));
	const plainError = await readSyncError(new Response('Payload too large', { status: 413 }));
	assert.match(plainError.message, /HTTP 413.*Payload too large/);
	const emptyError = await readSyncError(new Response(null, { status: 500 }));
	assert.match(emptyError.message, /no error details/);
	const unauthorized = await readSyncError(Response.json({ error: 'unauthorized' }, { status: 401 }));
	assert.match(unauthorized.message, /HTTP 401.*unauthorized/);

	globalThis.fetch = async () => Response.json({ success: true, data: { entries: [] } });
	const success = await proxyApiRequest(request(), 'sync', 'http://backend:8080');
	assert.equal(success.status, 200);
	assert.deepEqual(await success.json(), { success: true, data: { entries: [] } });
	assert.ok(success.headers.get('x-request-id'));
	assert.equal(logs.length, 2);
	for (const secret of ['token-secret', 'financial-secret', 'query-secret']) {
		assert.ok(!JSON.stringify(logs).includes(secret), `Logs must not include ${secret}`);
	}

	const protocol = await server.ssrLoadModule('/src/lib/sync-protocol.ts');
	const { syncFinanceState } = await server.ssrLoadModule('/src/lib/sync.ts');
	const settings = { id: 'settings', activeGroupId: 'group', deviceUserId: 'test-user', baseCurrency: 'SGD' };
	const record = (id, overrides = {}) => ({ id, groupId: 'group', updatedAt: '2026-10-01T00:00:00Z', createdAt: '2026-10-01T00:00:00Z', ...overrides });
	const records = {
		...protocol.emptySyncRecords(), groups: [record('group')], accounts: [record('account')], categories: [record('category')],
		entries: Array.from({ length: 1200 }, (_, i) => record(`entry-${String(i).padStart(4, '0')}`, { accountId: 'account', categoryId: 'category', note: '费用😀'.repeat(150) }))
	};
	const checkpoint = { id: 'test-user:group', fingerprints: {} };
	const batches = [...protocol.buildSyncBatches(settings, records)];
	assert.ok(batches.length > 12);
	assert.equal(batches.flatMap((batch) => batch.entries).length, 1200);
	for (const batch of batches) {
		assert.ok(protocol.syncPayloadBytes(batch) <= protocol.MAX_SYNC_BYTES);
		assert.ok(protocol.SYNC_COLLECTIONS.reduce((count, key) => count + batch[key].length, 0) <= 100);
	}
	assert.equal(batches.find(batch => batch.accounts.length).accounts[0].id, 'account');
	assert.equal(batches.find(batch => batch.categories.length).categories[0].id, 'category');
	assert.throws(() => [...protocol.buildSyncBatches(settings, { ...protocol.emptySyncRecords(), entries: [record('huge', { note: '😀'.repeat(40000) })] })], /exceeds the 128 KB/);
	await protocol.acknowledgeSyncRecords(checkpoint, records);
	const clean = await protocol.pendingSyncRecords(records, checkpoint);
	assert.equal([...protocol.buildSyncBatches(settings, clean)].length, 0);
	assert.equal(await protocol.recordFingerprint({ id: 'a', metadata: { z: 1, a: 2 } }), await protocol.recordFingerprint({ metadata: { a: 2, z: 1 }, id: 'a' }));
	// Neither a backwards clock nor a tombstone may be skipped.
	const edited = structuredClone(records);
	edited.entries[0].updatedAt = '2025-01-01T00:00:00Z';
	edited.entries[0].note = 'offline edit with backwards clock';
	edited.entries[1].deletedAt = '2026-10-02T00:00:00Z';
	assert.equal((await protocol.pendingSyncRecords(edited, checkpoint)).entries.length, 2);

	const harness = globalThis.__syncTest = { checkpoints: new Map(), requests: [], serverRecords: protocol.emptySyncRecords(), merged: protocol.emptySyncRecords(), pushCount: 0, failAt: 6, revision: 0 };
	const serverTime = () => new Date(Date.UTC(2026,9,6) + (++harness.revision)*1000).toISOString();
	const sameContent = (a,b) => {
		const clean = row => { const copy = structuredClone(row); for (const key of ['createdAt','updatedAt','createdBy','inviteCode']) delete copy[key]; return copy; };
		return protocol.sameSyncRecord(clean(a), clean(b));
	};
	harness.fetch = async (_url, init) => {
		const payload = JSON.parse(init.body);
		harness.requests.push(payload);
		await harness.beforeFetch?.(payload);
		if (payload.sync.mode === 'push') {
			if (++harness.pushCount === harness.failAt) return Response.json({ error: 'temporary failure' }, { status: 500 });
			// Model the server's private-category lookup before any write. An
			// incoming owner field must not grant access to an existing category.
			for (const row of payload.categories) {
				const stored = harness.serverRecords.categories.find(item => item.id === row.id);
				if (stored?.scope === 'user' && stored.ownerUserId !== (harness.userId ?? 'test-user')) {
					return Response.json({ error: 'sync record is not accessible' }, { status: 403 });
				}
			}
			const accepted = protocol.emptySyncRecords();
			const acceptedVersions = {};
			const conflicts = [];
			for (const key of protocol.SYNC_COLLECTIONS) {
				for (const row of payload[key]) {
					const index = harness.serverRecords[key].findIndex(item => item.id === row.id);
					const existing = harness.serverRecords[key][index];
					const version = existing ? await protocol.recordFingerprint(existing) : 'missing';
					const base = payload.sync.baseVersions?.[key]?.[row.id] ?? '';
					const equal = existing && sameContent(row, existing);
					if (!equal && (existing ? base !== version : base && base !== 'missing')) {
						conflicts.push({ collection:key, id:row.id, server:existing ?? null, serverVersion:version });
						continue;
					}
					let canonical = existing;
					if (!equal) {
						const timestamp = serverTime();
						canonical = { ...row,
							...(key === 'categories' && row.scope === 'user' ? { ownerUserId: harness.userId ?? 'test-user' } : {}),
							createdAt:existing?.createdAt ?? timestamp, updatedAt:timestamp };
						if (index < 0) harness.serverRecords[key].push(canonical); else harness.serverRecords[key][index] = canonical;
					}
					accepted[key].push(canonical);
					(acceptedVersions[key] ??= {})[row.id] = await protocol.recordFingerprint(canonical);
				}
			}
			return Response.json({ success:true, data:{ ...accepted, acceptedVersions, conflicts, protocolVersion:3, settings, syncedAt:serverTime() } });
		}
		const { collection, cursor, limit, known = {} } = payload.sync;
		const rows = harness.serverRecords[collection].filter(row =>
			(!cursor || row.id > cursor) && (collection !== 'categories' || row.scope !== 'user' || row.ownerUserId === (harness.userId ?? 'test-user'))
		).sort((a,b) => a.id.localeCompare(b.id));
		const pageRows = rows.slice(0,limit);
		const hasMore = rows.length > limit;
		const versions = Object.fromEntries(await Promise.all(pageRows.map(async row => [row.id, await protocol.recordFingerprint(row)])));
		const changedRows = pageRows.filter(row => known[row.id] !== versions[row.id]);
		return Response.json({ success:true, data:{ ...protocol.emptySyncRecords(), [collection]:changedRows, protocolVersion:3, settings,
			page:{ collection, hasMore, versions, nextCursor:hasMore ? pageRows.at(-1).id : undefined }, syncedAt:serverTime() } });
	};
	const callSync = async (state, resolve) => {
		harness.localState = state;
		harness.merged = protocol.emptySyncRecords();
		const result = await syncFinanceState(state, () => harness.localState, undefined, resolve,
			(remote, expected, removed) => { harness.localState = protocol.mergeServerState(harness.localState, remote, expected, removed); });
		harness.localState = result;
		return result;
	};
	const state = { settings, ...records };
	await assert.rejects(callSync(state), /HTTP 500.*temporary failure/);
	const acknowledgedIds = new Set(harness.requests.slice(0,5).flatMap(batch => batch.entries.map(row => row.id)));
	assert.ok(acknowledgedIds.size > 0);
	harness.failAt = 0;
	harness.requests = [];
	const synced = await callSync(harness.localState);
	assert.equal(synced.entries.length,1200);
	for (const batch of harness.requests.filter(item => item.sync.mode === 'push')) assert.ok(batch.entries.every(row => !acknowledgedIds.has(row.id)), 'retry resent an acknowledged record');
	assert.equal(harness.requests.filter(item => item.sync.collection === 'entries').length,12);
	harness.requests = [];
	await callSync(synced);
	assert.equal(harness.requests.filter(item => item.sync.mode === 'push').length,0);
	assert.equal(harness.merged.entries.length,0, 'unchanged transactions should not be downloaded');
	harness.serverRecords.entries[400].note = 'server-side canonical change';
	const changedSynced = await callSync(synced);
	assert.equal(harness.merged.entries.length,1, 'only the changed transaction should be downloaded');
	assert.equal(harness.merged.entries[0].note,'server-side canonical change');
	const backwards = structuredClone(changedSynced);
	backwards.entries[0].updatedAt = '2025-01-01T00:00:00Z';
	backwards.entries[0].note = 'valid edit with a backwards device clock';
	const corrected = await callSync(backwards);
	assert.equal(corrected.entries[0].note,backwards.entries[0].note);
	assert.ok(corrected.entries[0].updatedAt > '2026-10-01', 'the server must assign the canonical timestamp');

	// Make an edit after sync starts; it must remain pending, even if its clock goes backwards.
	const lateEdit = structuredClone(corrected);
	lateEdit.entries[0].note = 'edited while sync was in flight';
	lateEdit.entries[0].updatedAt = '2024-01-01T00:00:00Z';
	harness.beforeFetch = payload => { if (payload.sync.mode === 'pull' && payload.sync.collection === 'groups') harness.localState = lateEdit; };
	const concurrent = await callSync(corrected);
	delete harness.beforeFetch;
	assert.equal(concurrent.entries[0].note,lateEdit.entries[0].note);
	assert.equal((await protocol.pendingSyncRecords(concurrent,harness.checkpoints.get('test-user:group'))).entries.length,1);

	let live = await callSync(concurrent);
	const serverRow = id => harness.serverRecords.entries.find(row => row.id === id);
	const id = live.entries[0].id;
	const localEdit = (state, note) => { const next = structuredClone(state); next.entries.find(row => row.id === id).note = note; next.entries.find(row => row.id === id).updatedAt = '2050-01-01T00:00:00Z'; return next; };
	const remoteEdit = note => { serverRow(id).note = note; serverRow(id).updatedAt = serverTime(); };
	let prompts = 0;
	remoteEdit('edited on another device');
	live = await callSync(localEdit(live,'my local edit'), async conflict => {
		prompts++;
		assert.equal(conflict.local.note,'my local edit');
		assert.equal(conflict.server.note,'edited on another device');
		return 'server';
	});
	assert.equal(prompts,1);
	assert.equal(live.entries.find(row => row.id === id).note,'edited on another device');
	assert.ok(live.entries.find(row => row.id === id).updatedAt < '2050', 'server choice must override a newer device clock');

	remoteEdit('another remote edit');
	live = await callSync(localEdit(live,'keep this local change'), async () => 'local');
	assert.equal(serverRow(id).note,'keep this local change');
	assert.equal(live.entries.find(row => row.id === id).note,'keep this local change');

	// The server changes after the popup opens: the first local choice must be rechecked.
	remoteEdit('before popup');
	prompts = 0;
	live = await callSync(localEdit(live,'my racing edit'), async conflict => {
		if (++prompts === 1) { remoteEdit('changed while popup was open'); return 'local'; }
		assert.equal(conflict.server.note,'changed while popup was open');
		return 'server';
	});
	assert.equal(prompts,2);
	assert.equal(serverRow(id).note,'changed while popup was open');

	remoteEdit('remote edit pending review');
	const deferred = localEdit(live,'local edit pending review');
	await assert.rejects(callSync(deferred, async () => 'later'), protocol.SyncConflictDeferredError);
	assert.equal(harness.localState.entries.find(row => row.id === id).note,'local edit pending review');
	assert.equal(serverRow(id).note,'remote edit pending review');
	await assert.rejects(callSync(harness.localState), protocol.SyncConflictDeferredError);
	live = await callSync(harness.localState, async () => 'server');

	// A new local edit made during the popup requires a fresh decision.
	remoteEdit('conflicting server edit');
	prompts = 0;
	live = await callSync(localEdit(live,'old local choice'), async conflict => {
		if (++prompts === 1) { harness.localState = localEdit(harness.localState,'new local edit during popup'); return 'server'; }
		assert.equal(conflict.local.note,'new local edit during popup');
		return 'local';
	});
	assert.equal(prompts,2);
	assert.equal(serverRow(id).note,'new local edit during popup');

	serverRow(id).deletedAt = serverTime();
	prompts = 0;
	live = await callSync(localEdit(live,'edited a remotely deleted transaction'), async conflict => {
		prompts++;
		assert.ok(conflict.server.deletedAt);
		return 'server';
	});
	assert.equal(prompts,1);
	assert.ok(live.entries.find(row => row.id === id).deletedAt);

	const removed = localEdit(live,'edit of a hard-removed record');
	harness.serverRecords.entries = harness.serverRecords.entries.filter(row => row.id !== id);
	live = await callSync(removed, async conflict => { assert.equal(conflict.server,null); return 'server'; });
	assert.ok(!live.entries.some(row => row.id === id));

	const accountSwitch = structuredClone(live);
	accountSwitch.entries[0].note = 'upload before changing account';
	harness.beforeFetch = payload => { if (payload.sync.mode === 'push') harness.userId = 'another-user'; };
	await assert.rejects(callSync(accountSwitch), /sign-in changed during sync/);
	delete harness.beforeFetch;
	delete harness.userId;

	// A usual-account login can encounter old private categories in IndexedDB.
	// Keep their local edits without uploading, removing, or claiming ownership.
	const otherPersonal = record('other-personal', { name: 'Private category', scope: 'user', ownerUserId: 'other-user' });
	const myPersonal = record('my-personal', { scope: 'user', ownerUserId: 'test-user' });
	const unassignedPersonal = record('unassigned-personal', { scope: 'user', ownerUserId: null });
	const shared = record('shared-category', { scope: 'household', ownerUserId: null });
	harness.serverRecords = { ...protocol.emptySyncRecords(), categories: [structuredClone(otherPersonal)] };
	harness.checkpoints = new Map();
	harness.requests = [];
	const cached = { settings, ...protocol.emptySyncRecords(), categories: [
		{ ...otherPersonal, name: 'Retain this offline edit' }, myPersonal, unassignedPersonal, shared
	] };
	const privateSynced = await callSync(cached);
	assert.equal(privateSynced.categories.find(row => row.id === otherPersonal.id).name, 'Retain this offline edit');
	assert.equal(harness.serverRecords.categories.find(row => row.id === otherPersonal.id).name, otherPersonal.name);
	assert.ok(harness.serverRecords.categories.some(row => row.id === myPersonal.id));
	assert.equal(privateSynced.categories.find(row => row.id === unassignedPersonal.id).ownerUserId, 'test-user', 'A new local category without an owner can still be assigned by the server');
	assert.ok(harness.serverRecords.categories.some(row => row.id === shared.id));
	assert.ok(harness.requests.filter(payload => payload.sync.mode === 'push').every(payload => payload.categories.every(row => row.id !== otherPersonal.id)));
	assert.equal(harness.checkpoints.get('test-user:group').fingerprints.categories?.[otherPersonal.id], undefined, 'Excluded edits must not be marked as synced');
	// Older checkpoints can remember a category that has since become private.
	// Its absence from this user's pull must not trigger removal or a popup.
	const olderCheckpoint = harness.checkpoints.get('test-user:group');
	olderCheckpoint.fingerprints.categories[otherPersonal.id] = await protocol.recordFingerprint(otherPersonal);
	(olderCheckpoint.baseVersions.categories ??= {})[otherPersonal.id] = await protocol.recordFingerprint(otherPersonal);
	harness.requests = [];
	const retried = await callSync(privateSynced);
	assert.equal(retried.categories.find(row => row.id === otherPersonal.id).name, 'Retain this offline edit');
	assert.equal(harness.checkpoints.get('test-user:group').fingerprints.categories[otherPersonal.id], olderCheckpoint.fingerprints.categories[otherPersonal.id], 'Old checkpoints must not acknowledge an excluded local edit');
	assert.equal(harness.requests.filter(payload => payload.sync.mode === 'push').length, 0, 'Cached private records must not keep retrying');

	console.log('Sync batching, server authority, conflict choices, race protection, retry, and diagnostics tests passed');
} finally {
	globalThis.fetch = originalFetch;
	console.error = originalConsoleError;
	delete globalThis.__syncTest;
	await server.close();
}

import { beforeEach, expect, it, vi } from 'vitest';

const mocks = vi.hoisted(() => ({ json: vi.fn(), fetch: vi.fn() }));
vi.mock('$lib/api/client', () => ({
	normalizeOrigin: (origin: string) => origin,
	ApiError: class ApiError extends Error {
		constructor(
			public status: number,
			public code: string
		) {
			super(code);
		}
	},
	apiJson: (...args: unknown[]) => mocks.json(...args),
	apiFetch: (...args: unknown[]) => mocks.fetch(...args)
}));
vi.mock('$lib/api/snapshots', () => ({ invalidateCircleSnapshots: vi.fn() }));

beforeEach(() => {
	vi.resetModules();
	mocks.json.mockReset();
	mocks.fetch.mockReset();
	vi.stubGlobal('navigator', { onLine: false });
});

async function queued(name: string, completedFile = false) {
	const queue = await import('./queue');
	const db = await import('$lib/idb/db');
	const file = {
		name: 'original.bin',
		type: 'application/octet-stream',
		size: 4,
		data: new ArrayBuffer(4)
	};
	const id = await queue.enqueuePost(
		`https://${name}.test`,
		name,
		{ body: 'original text', entry_date: '2026-11-04' },
		completedFile ? [file, file] : [file]
	);
	const item = (await db.getQueueItem(id))!;
	await db.putQueueItem(id, {
		...item,
		uploads: completedFile
			? [
					{ file_index: 0, session_id: 'done', blob_id: 'saved-blob' },
					{ file_index: 1, session_id: 'expired' }
				]
			: [{ file_index: 0, session_id: 'expired' }]
	});
	return { queue, db, id, item };
}

async function responding(expired: 'saved' | 'always' = 'saved') {
	const { ApiError } = await import('$lib/api/client');
	mocks.json.mockImplementation(async (_origin: string, path: string) => ({
		id: path === '/uploads' ? 'fresh' : path.endsWith('/complete') ? 'fresh-blob' : 'post'
	}));
	mocks.fetch.mockImplementation(
		async (_origin: string, path: string, init: { method: string }) => {
			if (expired === 'always' || path === '/uploads/expired') throw new ApiError(404, 'not_found');
			if (init.method === 'HEAD') return { headers: new Headers({ 'Upload-Offset': '0' }) };
			return { json: async () => ({ received_bytes: 4 }) };
		}
	);
}

it('recreates an expired session and preserves completed files and client_id', async () => {
	const { queue, db, id, item } = await queued('expired-recovery', true);
	await responding();
	vi.stubGlobal('navigator', { onLine: true });
	await queue.drainQueue();
	expect(await db.getQueueItem(id)).toBeUndefined();
	expect(mocks.json.mock.calls.filter((c) => c[1] === '/uploads')).toHaveLength(1);
	expect(mocks.fetch.mock.calls.some((c) => c[1] === '/uploads/done')).toBe(false);
	const create = mocks.json.mock.calls.find((c) => c[1].endsWith('/posts'))!;
	const body = JSON.parse(create[2].body);
	expect(body.client_id).toBe(item.client_id);
	expect(body.client_id).toBeTruthy();
	expect(body.body).toBe('original text');
	expect(body.media.map((m: { blob_id: string }) => m.blob_id)).toEqual([
		'saved-blob',
		'fresh-blob'
	]);
});

it('bounds persistent 404s, retains the original, and lets manual retry start fresh', async () => {
	const { queue, db, id, item } = await queued('expired-bounded');
	await responding('always');
	vi.stubGlobal('navigator', { onLine: true });
	await queue.drainQueue();
	const failed = (await db.getQueueItem(id))!;
	expect(failed).toMatchObject({ state: 'failed', error: 'not_found', uploads: [] });
	expect(failed.files).toEqual(item.files);
	expect(failed.payload).toEqual(item.payload);
	expect(mocks.fetch).toHaveBeenCalledTimes(2);
	vi.stubGlobal('navigator', { onLine: false });
	await queue.retryQueueItem(id);
	expect(await db.getQueueItem(id)).toMatchObject({ state: 'pending', attempts: 0 });
	await responding();
	// Recreate the module to model a browser restart with persistent IndexedDB.
	vi.resetModules();
	vi.stubGlobal('navigator', { onLine: true });
	await (await import('./queue')).drainQueue();
	expect(await db.getQueueItem(id)).toBeUndefined();
});

it('does not treat a missing post or circle as an expired upload', async () => {
	const { queue, db, id } = await queued('missing-circle');
	await responding();
	const { ApiError } = await import('$lib/api/client');
	mocks.json.mockImplementation(async (_origin: string, path: string) => {
		if (path.endsWith('/posts')) throw new ApiError(404, 'not_found');
		return { id: path === '/uploads' ? 'fresh' : 'fresh-blob' };
	});
	vi.stubGlobal('navigator', { onLine: true });
	await queue.drainQueue();
	expect(await db.getQueueItem(id)).toMatchObject({
		state: 'failed',
		uploads: [{ blob_id: 'fresh-blob' }]
	});
	expect(mocks.json.mock.calls.filter((c) => c[1] === '/uploads')).toHaveLength(1);
});

it('retries a lost completion response with the same post client_id', async () => {
	const { queue, db, id, item } = await queued('lost-upload-complete');
	await responding();
	let completed = false;
	let sessions = 0;
	const { ApiError } = await import('$lib/api/client');
	mocks.json.mockImplementation(async (_origin: string, path: string) => {
		if (path === '/uploads') return { id: `fresh-${++sessions}` };
		if (path.endsWith('/complete') && !completed) {
			completed = true;
			throw new TypeError('response lost');
		}
		return { id: path.endsWith('/complete') ? 'fresh-blob' : 'post' };
	});
	mocks.fetch.mockImplementation(
		async (_origin: string, path: string, init: { method: string }) => {
			if (path === '/uploads/expired' || (completed && path === '/uploads/fresh-1'))
				throw new ApiError(404, 'not_found');
			return init.method === 'HEAD'
				? { headers: new Headers() }
				: { json: async () => ({ received_bytes: 4 }) };
		}
	);
	vi.stubGlobal('navigator', { onLine: true });
	await queue.drainQueue();
	expect(await db.getQueueItem(id)).toMatchObject({ state: 'pending' });
	await queue.drainQueueFor(item.origin);
	expect(await db.getQueueItem(id)).toBeUndefined();
	const creates = mocks.json.mock.calls.filter((c) => c[1].endsWith('/posts'));
	expect(creates).toHaveLength(1);
	expect(JSON.parse(creates[0][2].body).client_id).toBe(item.client_id);
	expect(sessions).toBe(2);
});

import { beforeEach, expect, it, vi } from 'vitest';
import { deleteQueueItem, getQueueItem, putQueueItem } from '$lib/idb/db';

const apiJson = vi.hoisted(() => vi.fn());
vi.mock('$lib/api/client', () => ({
	apiJson,
	ApiError: class ApiError extends Error {}
}));
vi.mock('$lib/api/snapshots', () => ({ invalidateCircleSnapshots: vi.fn() }));

const origin = 'https://recovery.test';
const payload = { body: 'Из очереди', entry_date: '2026-11-19' };
const network = { onLine: false };

beforeEach(() => {
	vi.resetModules();
	apiJson.mockReset().mockResolvedValue({});
	network.onLine = false;
	vi.stubGlobal('navigator', network);
});

it('reclaims a recent upload after restart only when the queue lock is granted', async () => {
	const { enqueuePost } = await import('./queue');
	const id = await enqueuePost(origin, 'circle', payload);
	try {
		const record = (await getQueueItem(id))!;
		await putQueueItem(id, { ...record, state: 'uploading', uploading_at: Date.now() });

		vi.resetModules();
		const { drainQueue } = await import('./queue');
		let grant = false;
		const request = vi.fn(async (_name, _options, run) => {
			await run(grant ? { name: 'wynd-queue' } : null);
		});
		vi.stubGlobal('navigator', { onLine: true, locks: { request } });
		await drainQueue();
		expect(await getQueueItem(id)).toMatchObject({ state: 'uploading' });
		expect(apiJson).not.toHaveBeenCalled();

		grant = true;
		await drainQueue();
		expect(apiJson).toHaveBeenCalledTimes(1);
		expect(JSON.parse(apiJson.mock.calls[0][2].body).client_id).toBe(record.client_id);
		expect(await getQueueItem(id)).toBeUndefined();
	} finally {
		await deleteQueueItem(id);
	}
});

it('cancels a stalled request offline and resumes the same post when the connection returns', async () => {
	const { drainQueue, enqueuePost, initQueueDrain } = await import('./queue');
	const id = await enqueuePost(origin, 'circle', payload);
	let release: (value: object) => void;
	const response = new Promise<object>((resolve) => {
		release = resolve;
	});
	let started: (options: RequestInit) => void;
	const request = new Promise<RequestInit>((resolve) => {
		started = resolve;
	});
	apiJson.mockImplementationOnce((_origin, _path, options: RequestInit) => {
		started(options);
		return new Promise((resolve, reject) => {
			options.signal?.addEventListener('abort', () => reject(options.signal?.reason), {
				once: true
			});
			void response.then(resolve);
		});
	});

	network.onLine = true;
	const cleanup = initQueueDrain();
	try {
		const options = await request;
		expect(await getQueueItem(id)).toMatchObject({ state: 'uploading' });
		network.onLine = false;
		window.dispatchEvent(new Event('offline'));
		expect(options.signal?.aborted).toBe(true);

		// Return online before the interrupted drain can write the pending state.
		network.onLine = true;
		window.dispatchEvent(new Event('online'));
		await drainQueue();

		expect(apiJson).toHaveBeenCalledTimes(2);
		const first = JSON.parse(apiJson.mock.calls[0][2].body);
		const retry = JSON.parse(apiJson.mock.calls[1][2].body);
		expect(first.client_id).toBeTruthy();
		expect(retry).toEqual(first);
		expect(await getQueueItem(id)).toBeUndefined();
	} finally {
		cleanup();
		release!({});
		await drainQueue();
		await deleteQueueItem(id);
	}
});

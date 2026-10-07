import { beforeEach, describe, expect, it, vi } from 'vitest';

const apiJson = vi.fn();

vi.mock('$lib/api/client', () => ({
	ApiError: class ApiError extends Error {
		status: number;
		code: string;
		constructor(status: number, code: string) {
			super(code);
			this.status = status;
			this.code = code;
		}
	},
	apiFetch: vi.fn(),
	apiJson: (...args: unknown[]) => apiJson(...args)
}));

vi.mock('$lib/api/snapshots', () => ({
	invalidateCircleSnapshots: vi.fn().mockResolvedValue(undefined)
}));

describe('queue', () => {
	beforeEach(() => {
		vi.resetModules();
		apiJson.mockReset();
		apiJson.mockResolvedValue({});
		vi.stubGlobal('navigator', { ...navigator, onLine: false });
	});

	it('enqueues and lists draft posts', async () => {
		const { enqueuePost, listQueuedPosts, CHUNK_SIZE } = await import('./queue');
		expect(CHUNK_SIZE).toBe(1024 * 1024);

		const id = await enqueuePost('https://srv.test', 'circle1', {
			body: 'hello',
			entry_date: '2026-09-02'
		});
		const posts = await listQueuedPosts('https://srv.test', 'circle1');
		expect(posts).toEqual([
			{
				id,
				body: 'hello',
				entry_date: '2026-09-02',
				file_count: 0,
				media: [],
				state: 'pending'
			}
		]);
	});

	// Лента рисует запись в очереди тем, что в ней лежит, а не пустой плиткой.
	it('describes queued media by kind', async () => {
		const { queuedMediaViews } = await import('./queue');
		const data = new ArrayBuffer(4);
		const poster = { type: 'image/jpeg', data: new ArrayBuffer(2) };
		const views = queuedMediaViews(
			[
				{ name: 'a.jpg', type: 'image/jpeg', size: 4, data },
				{ name: 'v.mp4', type: 'video/mp4', size: 4, data },
				{ name: 'voice.m4a', type: 'audio/mp4', size: 4, data },
				{ name: 'list.pdf', type: 'application/pdf', size: 4, data }
			],
			[
				{ kind: 'photo' },
				{ kind: 'video', video_poster: poster },
				{ kind: 'attachment', voice: true, audio_duration_ms: 48000, audio_peaks: [10, 90] },
				{ kind: 'attachment' }
			]
		);
		expect(views.map((v) => v.kind)).toEqual(['photo', 'video', 'attachment', 'attachment']);
		expect(views[0].preview?.data).toBe(data);
		expect(views[1].preview).toBe(poster);
		expect(views[2]).toMatchObject({ voice: true, duration_ms: 48000, peaks: [10, 90] });
		expect(views[2].preview).toBeUndefined();
		expect(views[3]).toMatchObject({ name: 'list.pdf', size: 4 });
		// Старые записи очереди без описания — фото, как их и отправит очередь.
		expect(queuedMediaViews([{ name: 'x', type: '', size: 4, data }], undefined)[0].kind).toBe('photo');
	});

	it('updates and removes queued posts', async () => {
		const { enqueuePost, updateQueuedPost, removeQueueItem, listQueuedPosts } =
			await import('./queue');
		const id = await enqueuePost('https://srv.test', 'c1', {
			body: 'draft',
			entry_date: '2026-09-01'
		});
		await updateQueuedPost(id, { body: 'edited', entry_date: '2026-09-02' });
		expect((await listQueuedPosts('https://srv.test', 'c1'))[0].body).toBe('edited');
		await removeQueueItem(id);
		expect(await listQueuedPosts('https://srv.test', 'c1')).toHaveLength(0);
	});

	it('collects failed items with errors', async () => {
		const { getFailedItems } = await import('./queue');
		const failed = getFailedItems([
			{
				id: 1,
				type: 'post',
				origin: '',
				circle_id: 'c',
				payload: { body: '', entry_date: '2026-01-01' },
				files: [],
				state: 'failed',
				error: 'edit_window_closed',
				created_at: 0
			},
			{
				id: 2,
				type: 'comment',
				origin: '',
				circle_id: 'c',
				payload: { post_id: 'p', body: 'x' },
				files: [],
				state: 'pending',
				created_at: 0
			}
		]);
		expect(failed).toHaveLength(1);
		expect(failed[0].error).toBe('edit_window_closed');
	});

	it('drains a comment when online', async () => {
		vi.stubGlobal('navigator', { ...navigator, onLine: true });
		const { enqueueComment, drainQueue, listQueueForCircle } = await import('./queue');
		const origin = 'https://drain.test';
		await enqueueComment(origin, 'circle-drain', { post_id: 'post1', body: 'привет' });
		expect(await listQueueForCircle(origin, 'circle-drain')).toHaveLength(1);
		await drainQueue();
		expect(apiJson).toHaveBeenCalledWith(
			origin,
			'/circles/circle-drain/posts/post1/comments',
			expect.objectContaining({ method: 'POST' })
		);
		expect(await listQueueForCircle(origin, 'circle-drain')).toHaveLength(0);
	});
	it('загрузка из очереди несёт имя файла', async () => {
		vi.stubGlobal('navigator', { ...navigator, onLine: true });
		apiJson.mockResolvedValue({ id: 'session1' });
		const { enqueuePost, drainQueue } = await import('./queue');
		const data = new ArrayBuffer(4);
		await enqueuePost(
			'https://name.test',
			'c-name',
			{ body: '', entry_date: '2026-10-02', media_meta: [{ kind: 'attachment' }] },
			[{ name: 'Голосовое.m4a', type: 'audio/mp4', size: 4, data }]
		);
		await drainQueue();
		const create = apiJson.mock.calls.find((c) => c[1] === '/uploads');
		expect(create).toBeTruthy();
		expect(JSON.parse(String(create![2].body)).filename).toBe('Голосовое.m4a');
	});
});

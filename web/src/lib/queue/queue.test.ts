import { beforeEach, describe, expect, it, vi } from 'vitest';

const apiJson = vi.fn();
const apiFetch = vi.fn();

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
	apiFetch: (...args: unknown[]) => apiFetch(...args),
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
		apiFetch.mockReset();
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

	// Запись в очереди подписана тем, что с ней происходит: ждёт или уходит.
	// Процент — только когда его есть чем измерить.
	it('labels a queued item by its send state', async () => {
		const { queuedTimeLabel } = await import('./queue');
		expect(queuedTimeLabel('pending')).toBe('в очереди');
		expect(queuedTimeLabel('failed', 0.5)).toBe('в очереди');
		expect(queuedTimeLabel('uploading')).toBe('отправляется');
		expect(queuedTimeLabel('uploading', 0)).toBe('отправляется · 0%');
		expect(queuedTimeLabel('uploading', 0.428)).toBe('отправляется · 42%');
		expect(queuedTimeLabel('uploading', 1)).toBe('отправляется · 100%');
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

	// Ролик уходит порциями по мегабайту: подписчик видит долю после каждой,
	// а когда запись ушла — что следить больше не за чем.
	it('сообщает ход отправки по порциям', async () => {
		vi.stubGlobal('navigator', { ...navigator, onLine: true });
		apiJson.mockResolvedValue({ id: 'session1' });
		apiFetch.mockImplementation(
			async (_origin: string, _path: string, init: { method: string; headers?: Record<string, string>; body?: ArrayBuffer }) => {
				if (init.method === 'HEAD') return { headers: new Headers({ 'Upload-Offset': '0' }) };
				const received = Number(init.headers!['Upload-Offset']) + init.body!.byteLength;
				return { json: async () => ({ received_bytes: received }) };
			}
		);
		const { enqueuePost, drainQueue, subscribeQueueProgress, CHUNK_SIZE } = await import('./queue');
		const seen: (number | undefined)[] = [];
		const unsub = subscribeQueueProgress((_id, fraction) => seen.push(fraction));
		const size = CHUNK_SIZE * 2 + CHUNK_SIZE / 2;
		await enqueuePost(
			'https://progress.test',
			'c-progress',
			{ body: '', entry_date: '2026-10-08', media_meta: [{ kind: 'video' }] },
			[{ name: 'v.mp4', type: 'video/mp4', size, data: new ArrayBuffer(size) }]
		);
		await drainQueue();
		unsub();
		expect(seen).toEqual([0, 0.4, 0.8, 1, undefined]);
	});

	// Мелкое уходит одной порцией: доли нет, подпись — просто «отправляется».
	it('не считает проценты у файла в одну порцию', async () => {
		vi.stubGlobal('navigator', { ...navigator, onLine: true });
		apiJson.mockResolvedValue({ id: 'session1' });
		apiFetch.mockImplementation(async (_o: string, _p: string, init: { method: string; body?: ArrayBuffer }) =>
			init.method === 'HEAD'
				? { headers: new Headers() }
				: { json: async () => ({ received_bytes: init.body!.byteLength }) }
		);
		const { enqueuePost, drainQueue, subscribeQueueProgress } = await import('./queue');
		const seen: (number | undefined)[] = [];
		const unsub = subscribeQueueProgress((_id, fraction) => seen.push(fraction));
		await enqueuePost(
			'https://small.test',
			'c-small',
			{ body: '', entry_date: '2026-10-08', media_meta: [{ kind: 'attachment', voice: true }] },
			[{ name: 'voice.m4a', type: 'audio/mp4', size: 4, data: new ArrayBuffer(4) }]
		);
		await drainQueue();
		unsub();
		expect(seen).toEqual([]);
	});
});

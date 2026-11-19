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
		// Места нет (C42, 3.12): запись ждёт, и в подписи сказано почему.
		expect(queuedTimeLabel('pending', undefined, 'quota_exceeded')).toBe(
			'в очереди · в круге кончилось место'
		);
		expect(queuedTimeLabel('pending', undefined, 'quota_exceeded:server')).toBe(
			'в очереди · на сервере кончилось место'
		);
		expect(queuedTimeLabel('uploading', undefined, 'quota_exceeded')).toBe('отправляется');
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
		expect(queuedMediaViews([{ name: 'x', type: '', size: 4, data }], undefined)[0].kind).toBe(
			'photo'
		);
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

	// Запись уходит из очереди, когда лента её уже перечитала: иначе карточка
	// очереди пропадала на 100%, а запись появлялась только с обновлением.
	it('даёт ленте перечитать круг до того, как снять запись', async () => {
		vi.stubGlobal('navigator', { ...navigator, onLine: true });
		const { enqueuePost, drainQueue, listQueueForCircle, subscribePostSent } =
			await import('./queue');
		const origin = 'https://sent.test';
		const stillQueued: number[] = [];
		const unsub = subscribePostSent(async (o, c) => {
			stillQueued.push((await listQueueForCircle(o, c)).length);
		});
		await enqueuePost(origin, 'circle-sent', { body: 'видео', entry_date: '2026-10-22' });
		await drainQueue();
		unsub();
		expect(stillQueued).toEqual([1]);
		expect(await listQueueForCircle(origin, 'circle-sent')).toHaveLength(0);
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
			async (
				_origin: string,
				_path: string,
				init: { method: string; headers?: Record<string, string>; body?: ArrayBuffer }
			) => {
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
		apiFetch.mockImplementation(
			async (_o: string, _p: string, init: { method: string; body?: ArrayBuffer }) =>
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

describe('queue retry schedule', () => {
	beforeEach(() => {
		vi.resetModules();
	});

	it('counts down the retry pause and skips failed items', async () => {
		const { nextRetryDelay } = await import('./queue');
		const now = 1_000_000;
		const base = { origin: '', circle_id: 'c', payload: { body: '', entry_date: '' }, files: [] };
		expect(nextRetryDelay([], now)).toBeUndefined();
		expect(
			nextRetryDelay(
				[{ ...base, id: 1, type: 'post', state: 'failed', created_at: now } as never],
				now
			)
		).toBeUndefined();
		// Первая неудача: пауза 30 с, прошло 10 с.
		expect(
			nextRetryDelay(
				[
					{
						...base,
						id: 2,
						type: 'post',
						state: 'pending',
						attempts: 1,
						created_at: now - 10_000
					} as never
				],
				now
			)
		).toBe(20_000);
		// Отправку прервали (переход/отмена): attempts=0, но повтор нужен.
		expect(
			nextRetryDelay(
				[
					{
						...base,
						id: 4,
						type: 'post',
						state: 'pending',
						attempts: 0,
						uploading_at: now - 5_000,
						created_at: now - 10_000
					} as never
				],
				now
			)
		).toBe(25_000);
		// Отправка, оставленная закрытой вкладкой: ждём до устаревания.
		expect(
			nextRetryDelay(
				[
					{
						...base,
						id: 3,
						type: 'post',
						state: 'uploading',
						uploading_at: now - 60_000,
						created_at: now - 60_000
					} as never
				],
				now
			)
		).toBe(60_000);
	});
});

describe('queue retries on its own', () => {
	beforeEach(() => {
		vi.resetModules();
		vi.stubGlobal('navigator', { ...navigator, onLine: true });
	});

	it('sends a queued post after the first transport failure, without an outside event', async () => {
		// Долгий повтор перехватываем: остальные таймеры (пауза ленты) живут.
		const scheduled: Array<{ fn: () => void; ms: number }> = [];
		const realSetTimeout = globalThis.setTimeout;
		const spy = vi.spyOn(globalThis, 'setTimeout').mockImplementation(((
			fn: () => void,
			ms?: number
		) => {
			if ((ms ?? 0) >= 10_000) {
				scheduled.push({ fn, ms: ms ?? 0 });
				return 0 as unknown as ReturnType<typeof setTimeout>;
			}
			return realSetTimeout(fn, ms);
		}) as typeof setTimeout);
		try {
			apiJson.mockRejectedValueOnce(new TypeError('Failed to fetch')).mockResolvedValue({});
			const { enqueuePost, drainQueue, listQueueForCircle } = await import('./queue');
			const origin = 'https://retry.test';
			await enqueuePost(origin, 'c-retry', { body: 'ждёт', entry_date: '2026-11-19' });
			await drainQueue();
			await new Promise((r) => realSetTimeout(r, 30));
			expect(await listQueueForCircle(origin, 'c-retry')).toHaveLength(1);
			expect(scheduled.length).toBeGreaterThan(0);

			const nowSpy = vi.spyOn(Date, 'now').mockReturnValue(Date.now() + 60_000);
			scheduled[0].fn();
			for (let i = 0; i < 50; i++) {
				await new Promise((r) => realSetTimeout(r, 10));
				if ((await listQueueForCircle(origin, 'c-retry')).length === 0) break;
			}
			expect(await listQueueForCircle(origin, 'c-retry')).toHaveLength(0);
			nowSpy.mockRestore();
		} finally {
			spy.mockRestore();
		}
	});
});

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { MEDIA_BATCH, resolveMediaUrls } from './batch';

const origin = 'https://srv.test';

// Каждый блоб отвечает, когда тест разрешит его «загрузку».
type Gate = { resolve: (url: string) => void; reject: (err: unknown) => void };
const gates = new Map<string, Gate>();
const started: string[] = [];

vi.mock('$lib/media/objectUrl', () => ({
	getMediaUrl: vi.fn((_origin: string, blobId: string) => {
		started.push(blobId);
		return new Promise<string>((resolve, reject) => {
			gates.set(blobId, { resolve, reject });
		});
	})
}));

function release(ids: string[]) {
	for (const id of ids) gates.get(id)?.resolve(`url:${id}`);
}

describe('resolveMediaUrls', () => {
	beforeEach(() => {
		vi.clearAllMocks();
		gates.clear();
		started.length = 0;
	});

	// Класс дефекта «экран не ждёт медиа» (замер 24.11: вкладка «Сетка»
	// держала индикатор, пока последовательный цикл не переберёт все
	// обложки — до 2000 подряд): готовая пачка применяется сразу,
	// не дожидаясь остальных пачек.
	it('applies a ready batch before later batches resolve', async () => {
		const ids = Array.from({ length: MEDIA_BATCH * 2 + 4 }, (_, i) => `b${i}`);
		const applied: string[] = [];
		const run = resolveMediaUrls(origin, ids, (id) => applied.push(id));

		expect(started).toHaveLength(MEDIA_BATCH);
		release(ids.slice(0, MEDIA_BATCH));
		await vi.waitFor(() => expect(applied).toHaveLength(MEDIA_BATCH));
		await vi.waitFor(() => expect(started).toHaveLength(MEDIA_BATCH * 2));
		// Вторая пачка уже началась, но не готова — первая на экране.
		expect(applied).toHaveLength(MEDIA_BATCH);

		// Пачка стартует, только когда готова прежняя — отпускаем по очереди.
		release(ids.slice(MEDIA_BATCH, MEDIA_BATCH * 2));
		await vi.waitFor(() => expect(started).toHaveLength(ids.length));
		release(ids.slice(MEDIA_BATCH * 2));
		await run;
		expect(applied).toHaveLength(ids.length);
	});

	// Неудачный блоб не роняет пачку: остальным адреса применяются,
	// неудачному зовётся onFail — как вёлся прежний цикл с «skip».
	it('applies the rest of a batch when one blob fails', async () => {
		const applied: string[] = [];
		const failed: string[] = [];
		const run = resolveMediaUrls(
			origin,
			['good-1', 'bad', 'good-2'],
			(id) => applied.push(id),
			MEDIA_BATCH,
			{
				onFail: (id) => failed.push(id)
			}
		);

		gates.get('bad')?.reject(new Error('no blob'));
		release(['good-1', 'good-2']);
		await run;

		expect([...applied].sort()).toEqual(['good-1', 'good-2']);
		expect(failed).toEqual(['bad']);
	});
});

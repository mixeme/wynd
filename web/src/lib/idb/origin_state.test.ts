import 'fake-indexeddb/auto';
import { beforeEach, describe, expect, it } from 'vitest';
import {
	addQueueItem,
	clearOriginState,
	getDb,
	listQueueItems,
	mediaKey,
	putMedia,
	putPin,
	type QueueRecord
} from './db';

function queueRecord(origin: string): QueueRecord {
	return {
		type: 'post',
		origin,
		circle_id: 'c1',
		payload: { body: 'офлайн', entry_date: '2026-08-30' },
		files: [],
		state: 'pending',
		created_at: Date.now()
	};
}

describe('clearOriginState', () => {
	beforeEach(async () => {
		const db = await getDb();
		await db.clear('queue');
		await db.clear('media');
		await db.clear('pins');
	});

	// Аудит 2026-09-22: после выхода на общем устройстве очередь уходила под
	// следующей учёткой, а фото оставались в IDB. Стирается только состояние
	// своего сервера.
	it('drops queue, media and pins of one origin only', async () => {
		const a = 'https://a.example';
		const b = 'https://b.example';
		await addQueueItem(queueRecord(a));
		await addQueueItem(queueRecord(b));
		await putMedia(mediaKey(a, 'blob-1'), { buffer: new ArrayBuffer(8), mime: 'image/jpeg' });
		await putMedia(mediaKey(b, 'blob-2'), { buffer: new ArrayBuffer(8), mime: 'image/jpeg' });
		await putPin(a, 'c1');
		await putPin(b, 'c1');

		await clearOriginState(a);

		const queue = await listQueueItems();
		expect(queue.map((q) => q.origin)).toEqual([b]);
		const db = await getDb();
		expect((await db.getAllKeys('media')) as string[]).toEqual([mediaKey(b, 'blob-2')]);
		expect(((await db.getAllKeys('pins')) as string[]).every((k) => k.startsWith(`${b}:`))).toBe(true);
	});
});

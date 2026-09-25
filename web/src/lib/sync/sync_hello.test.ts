import { describe, expect, it } from 'vitest';
import { getCursor, getSnapshot, putCursor, putSnapshot, snapshotKey } from '$lib/idb/db';
import { handleSyncFrame } from './sync';

const origin = 'https://wynd.example';
const listKey = snapshotKey(origin, 'circles', 'list');

// Инвариант (план 42, BKP-3): курсор больше max_seq сервера — сервер
// восстановлен из бэкапа; курсор и снимки сбрасываются. Курсор не больше —
// hello ничего не трогает.
describe('sync hello frame', () => {
	it('resets a stale cursor and drops snapshots', async () => {
		await putCursor(origin, 500);
		await putSnapshot(listKey, { data: {}, fetched_at: 1 });
		await handleSyncFrame(origin, '{"max_seq":42}');
		expect((await getCursor(origin))?.seq).toBe(42);
		expect(await getSnapshot(listKey)).toBeUndefined();
	});

	it('keeps a cursor that is not ahead of the server', async () => {
		await putCursor(origin, 40);
		await putSnapshot(listKey, { data: {}, fetched_at: 1 });
		await handleSyncFrame(origin, '{"max_seq":42}');
		expect((await getCursor(origin))?.seq).toBe(40);
		expect(await getSnapshot(listKey)).toBeDefined();
	});

	it('ignores frames that are neither an event nor hello', async () => {
		await putCursor(origin, 7);
		await handleSyncFrame(origin, '{"foo":1}');
		await handleSyncFrame(origin, 'not json');
		expect((await getCursor(origin))?.seq).toBe(7);
	});
});

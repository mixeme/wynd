import { describe, expect, it } from 'vitest';
import { getCursor, getSnapshot, putCursor, putSnapshot, snapshotKey } from '$lib/idb/db';
import { handleSyncFrame, registerRefetch } from './sync';

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

	// Кадр street: позвали в новый круг — список перечитывается сам; курсор
	// событий не трогается. Снимок остаётся: офлайн он единственное, что есть
	// (C105, C108).
	it('keeps the circles list on a street frame', async () => {
		await putCursor(origin, 9);
		await putSnapshot(listKey, { data: {}, fetched_at: 1 });
		let listRefetched = 0;
		let circleRefetched = 0;
		const offList = registerRefetch({ origin, refetch: async () => void listRefetched++ });
		const offCircle = registerRefetch({
			origin,
			circleId: 'c1',
			refetch: async () => void circleRefetched++
		});
		await handleSyncFrame(origin, '{"street":true}');
		offList();
		offCircle();
		expect(await getSnapshot(listKey)).toBeDefined();
		expect((await getCursor(origin))?.seq).toBe(9);
		expect(listRefetched).toBe(1);
		expect(circleRefetched).toBe(0);
	});

	// Событие круга: открытые экраны перечитываются сами, а снимки остаются —
	// иначе офлайн-настройки показывали только ошибку (C105), а холодная
	// ссылка на запись не находила круг (C108).
	it('keeps circle snapshots on an event frame', async () => {
		const settingsKey = snapshotKey(origin, 'settings', 'c1');
		await putCursor(origin, 9);
		await putSnapshot(listKey, { data: {}, fetched_at: 1 });
		await putSnapshot(settingsKey, { data: {}, fetched_at: 1 });
		let refetched = 0;
		let other = 0;
		const off = registerRefetch({ origin, circleId: 'c1', refetch: () => void refetched++ });
		const offOther = registerRefetch({ origin, circleId: 'c2', refetch: () => void other++ });
		await handleSyncFrame(
			origin,
			'{"seq":10,"circle_id":"c1","type":"post","id":"p1","is_service":false,"actor_name":"A","summary":"s","created_at":"2026-11-20T00:00:00Z","payload":{}}'
		);
		off();
		offOther();
		expect(await getSnapshot(listKey)).toBeDefined();
		expect(await getSnapshot(settingsKey)).toBeDefined();
		expect(refetched).toBe(1);
		expect(other).toBe(0);
	});

	it('ignores frames that are neither an event nor hello', async () => {
		await putCursor(origin, 7);
		await handleSyncFrame(origin, '{"foo":1}');
		await handleSyncFrame(origin, 'not json');
		expect((await getCursor(origin))?.seq).toBe(7);
	});
});

import 'fake-indexeddb/auto';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { getDb, putSession, putSnapshot, snapshotKey } from '$lib/idb/db';
import { loadStreetCached } from './circles';

// C97: улочка рисуется из снимков на устройстве, не дожидаясь ответов
// серверов. Свежие ряды приносит loadStreet следом — здесь только первый кадр.
const ORIGIN = '';

const ITEM = {
	id: 'c1',
	name: 'Дача',
	color: 'ochre',
	status: 'active',
	unread: 2,
	last_read_seq: 5,
	last_summary: 'Комментарий: Carol',
	last_at: '2026-11-19T04:51:01Z'
};

async function clear(): Promise<void> {
	const db = await getDb();
	const stores = ['sessions', 'snapshots', 'settings', 'groups', 'pins', 'street_ops'] as const;
	for (const store of stores) {
		await db.clear(store);
	}
}

beforeEach(async () => {
	await clear();
	await putSession({
		origin: ORIGIN,
		name: 'Стенд',
		email: 'anya@example.com',
		token: 't',
		account_id: 'a'
	});
});

describe('улочка из снимка (C97)', () => {
	it('строит ряды без единого запроса к сети', async () => {
		const fetchSpy = vi.spyOn(globalThis, 'fetch');
		await putSnapshot(snapshotKey(ORIGIN, 'circles', '_list'), {
			data: { circles: [ITEM], groups: [] },
			fetched_at: Date.now()
		});

		const street = await loadStreetCached();

		expect(street?.circles.map((c) => c.id)).toEqual(['c1']);
		expect(street?.circles[0].preview).toBe('Комментарий: Carol');
		expect(street?.circles[0].unread).toBe(2);
		// Признака в снимках нет — строка не выдумывает «есть другие».
		expect(street?.circles[0].hasOthers).toBeUndefined();
		expect(fetchSpy).not.toHaveBeenCalled();
		fetchSpy.mockRestore();
	});

	it('берёт признак «есть кто-то ещё» из снимка круга', async () => {
		await putSnapshot(snapshotKey(ORIGIN, 'circles', '_list'), {
			data: { circles: [ITEM], groups: [] },
			fetched_at: Date.now()
		});
		await putSnapshot(snapshotKey(ORIGIN, 'meta', 'c1'), {
			data: { has_others: false },
			fetched_at: Date.now()
		});

		const street = await loadStreetCached();

		expect(street?.circles[0].hasOthers).toBe(false);
	});

	it('без снимка не показывает пустую улочку', async () => {
		expect(await loadStreetCached()).toBeUndefined();
	});
});

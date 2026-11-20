import 'fake-indexeddb/auto';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { getDb, getSnapshot, putSnapshot, snapshotKey } from '$lib/idb/db';

// C108: отметка прочитанного раньше удаляла снимок улицы целиком, и холодное
// открытие ссылки на запись офлайн не могло найти круг: браузер уходил на
// пустую улицу. Теперь прочитанное отмечается в самом снимке.
vi.mock('$lib/api/client', async (importOriginal) => {
	const actual = await importOriginal<typeof import('$lib/api/client')>();
	return { ...actual, apiJson: vi.fn(async () => ({})) };
});

const ORIGIN = 'https://wynd.example';
const listKey = snapshotKey(ORIGIN, 'circles', '_list');

async function clear(): Promise<void> {
	const db = await getDb();
	await db.clear('snapshots');
}

beforeEach(clear);

describe('отметка прочитанного (C108)', () => {
	it('гасит «новое» в снимке, не удаляя его', async () => {
		await putSnapshot(listKey, {
			fetched_at: 1,
			data: {
				circles: [
					{ id: 'c1', name: 'Дача', status: 'active', unread: 3, last_read_seq: 10 },
					{ id: 'c2', name: 'Семья', status: 'active', unread: 1, last_read_seq: 5 }
				]
			}
		});
		const { advanceReadCursor } = await import('./read-cursor');
		await advanceReadCursor(ORIGIN, 'c1', 42);
		const record = await getSnapshot(listKey);
		expect(record).toBeDefined();
		const circles = (record!.data as { circles: Array<Record<string, unknown>> }).circles;
		expect(circles[0]).toMatchObject({ id: 'c1', unread: 0, last_read_seq: 42 });
		// Чужой круг не тронут.
		expect(circles[1]).toMatchObject({ id: 'c2', unread: 1, last_read_seq: 5 });
	});

	it('снимка нет — отметка ничего не создаёт', async () => {
		const { advanceReadCursor } = await import('./read-cursor');
		await advanceReadCursor(ORIGIN, 'c1', 42);
		expect(await getSnapshot(listKey)).toBeUndefined();
	});
});

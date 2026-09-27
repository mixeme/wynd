import 'fake-indexeddb/auto';
import { beforeEach, describe, expect, it } from 'vitest';
import {
	clearMediaStore,
	closeDb,
	getDb,
	invalidateSnapshots,
	mediaStoreBytes,
	putMedia,
	putSnapshot,
	snapshotKey
} from './db';

async function keys(): Promise<string[]> {
	const db = await getDb();
	return (await db.getAllKeys('snapshots')) as string[];
}

describe('invalidateSnapshots', () => {
	beforeEach(async () => {
		const db = await getDb();
		await db.clear('snapshots');
		await db.clear('media');
	});

	// Инвариант (CLI-1): по кругу без указания вида снимка уходят именно его
	// снимки. Прежняя сверка сравнивала `feed:<uuid>` с circleId и не
	// совпадала никогда — кэш исключённого круга оставался и читался офлайн.
	it('drops every kind of one circle and keeps other circles', async () => {
		const origin = 'https://a.example';
		const mine = '01a0-mine';
		const other = '01a0-other';
		for (const kind of ['feed', 'grid', 'days', 'map'] as const) {
			await putSnapshot(snapshotKey(origin, kind, mine), { data: {}, fetched_at: 1 });
			await putSnapshot(snapshotKey(origin, kind, other), { data: {}, fetched_at: 1 });
		}
		await putSnapshot(snapshotKey(origin, 'day', `${mine}:2026-08-30`), {
			data: {},
			fetched_at: 1
		});
		await putSnapshot(snapshotKey(origin, 'circles', 'all'), { data: {}, fetched_at: 1 });

		await invalidateSnapshots(origin, { circleId: mine });

		const left = await keys();
		expect(left.some((k) => k.includes(mine))).toBe(false);
		expect(left.filter((k) => k.includes(other))).toHaveLength(4);
		expect(left).toContain(snapshotKey(origin, 'circles', 'all'));
	});

	// Два сервера на одном хосте с разными портами — разные origin, и один
	// не должен чистить кэш другого.
	it('keeps snapshots of another server on the same host', async () => {
		const circleId = '01a0-mine';
		await putSnapshot(snapshotKey('https://a.example', 'feed', circleId), {
			data: {},
			fetched_at: 1
		});
		await putSnapshot(snapshotKey('https://a.example:8443', 'feed', circleId), {
			data: {},
			fetched_at: 1
		});

		await invalidateSnapshots('https://a.example', { circleId });

		expect(await keys()).toEqual([snapshotKey('https://a.example:8443', 'feed', circleId)]);
	});

	// Ключ с неизвестным видом трогать нельзя: вид в ключе обязателен.
	it('ignores keys whose segment is not a snapshot kind', async () => {
		const origin = 'https://a.example';
		const circleId = '01a0-mine';
		const db = await getDb();
		await db.put('snapshots', { data: {}, fetched_at: 1 }, `${origin}:${circleId}`);

		await invalidateSnapshots(origin, { circleId });

		expect(await keys()).toEqual([`${origin}:${circleId}`]);
	});
});

describe('mediaStoreBytes', () => {
	// Размер — по метаданным, которые пишет putMedia: сами файлы не читаются.
	it('sums sizes from media metadata', async () => {
		await clearMediaStore();
		await putMedia('a', { buffer: new ArrayBuffer(10), mime: 'image/jpeg' });
		await putMedia('b', { buffer: new ArrayBuffer(32), mime: 'image/jpeg' });
		expect(await mediaStoreBytes()).toBe(42);
		await closeDb();
	});
});

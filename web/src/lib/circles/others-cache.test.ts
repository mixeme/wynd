import 'fake-indexeddb/auto';
import { beforeEach, describe, expect, it } from 'vitest';
import { getDb } from '$lib/idb/db';
import { readHasOthers, readHasOthersMap, writeHasOthers } from './others-cache';

beforeEach(async () => {
	const db = await getDb();
	await db.clear('snapshots');
});

describe('признак «есть кто-то ещё» (3.17)', () => {
	it('без снимка неизвестен, после записи читается', async () => {
		expect(await readHasOthers('', 'c1')).toBeUndefined();
		await writeHasOthers('', 'c1', false);
		expect(await readHasOthers('', 'c1')).toBe(false);
		await writeHasOthers('', 'c1', true);
		expect(await readHasOthers('', 'c1')).toBe(true);
	});

	it('карта не смешивает серверы и круги', async () => {
		await writeHasOthers('', 'c1', false);
		await writeHasOthers('', 'c2', true);
		await writeHasOthers('https://other', 'c1', true);

		expect([...(await readHasOthersMap('')).entries()].sort()).toEqual([
			['c1', false],
			['c2', true]
		]);
		expect(await readHasOthersMap('https://other')).toEqual(new Map([['c1', true]]));
	});
});

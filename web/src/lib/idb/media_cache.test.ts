import 'fake-indexeddb/auto';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { clearMediaStore, getMedia, mediaStoreBytes, putMedia, trimMediaStore } from './db';

const bytes = (n: number) => ({ buffer: new ArrayBuffer(n), mime: 'image/webp' });

// У кэша медиа есть потолок (по умолчанию 2 ГБ): сверх него уходят давно не
// открытые файлы, свежие остаются. Размер считается по метаданным.
describe('media cache ceiling', () => {
	beforeEach(async () => {
		await clearMediaStore();
		// Только Date: на таймерах живёт сам fake-indexeddb.
		vi.useFakeTimers({ toFake: ['Date'] });
	});
	afterEach(() => vi.useRealTimers());

	it('evicts the least recently used files down to 90% of the ceiling', async () => {
		vi.setSystemTime(1000);
		await putMedia('o:old', bytes(1000));
		vi.setSystemTime(2000);
		await putMedia('o:mid', bytes(1000));
		vi.setSystemTime(3000);
		await putMedia('o:new', bytes(1000));
		// Открыли старый — теперь давнее всех «mid».
		vi.setSystemTime(4000);
		await getMedia('o:old');
		await new Promise((r) => setTimeout(r, 0));
		expect(await mediaStoreBytes()).toBe(3000);

		expect(await trimMediaStore(2500)).toBe(1);
		expect(await getMedia('o:mid')).toBeUndefined();
		expect(await getMedia('o:old')).toBeDefined();
		expect(await mediaStoreBytes()).toBe(2000);
	});

	it('leaves the cache alone under the ceiling', async () => {
		await putMedia('o:a', bytes(100));
		expect(await trimMediaStore(1000)).toBe(0);
		expect(await mediaStoreBytes()).toBe(100);
	});
});

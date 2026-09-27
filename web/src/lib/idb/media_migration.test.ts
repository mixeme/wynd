import 'fake-indexeddb/auto';
import { openDB } from 'idb';
import { describe, expect, it } from 'vitest';
import { getDb, mediaStoreBytes, trimMediaStore } from './db';

// Кэш, накопленный до версии 3, получает метаданные при обновлении базы —
// иначе потолок его бы не видел, и старые файлы лежали бы вечно.
describe('media cache migration v2 → v3', () => {
	it('backfills sizes of files cached before the ceiling existed', async () => {
		const old = await openDB('wynd', 2, {
			upgrade(db) {
				for (const store of ['sessions', 'admin_session', 'cursors', 'snapshots', 'media', 'pins', 'settings']) {
					if (store === 'sessions') db.createObjectStore(store, { keyPath: 'origin' });
					else if (store === 'cursors') db.createObjectStore(store, { keyPath: 'origin' });
					else db.createObjectStore(store);
				}
				db.createObjectStore('queue', { autoIncrement: true });
				db.createObjectStore('groups', { keyPath: 'id' });
			}
		});
		await old.put('media', { buffer: new ArrayBuffer(700), mime: 'image/webp' }, 'o:a');
		await old.put('media', { buffer: new ArrayBuffer(300), mime: 'image/webp' }, 'o:b');
		old.close();

		await getDb();
		expect(await mediaStoreBytes()).toBe(1000);
		// Старые файлы вытесняются первыми — used у них 0.
		expect(await trimMediaStore(500)).toBeGreaterThan(0);
		expect(await mediaStoreBytes()).toBeLessThanOrEqual(450);
	});
});

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { getMedia, mediaKey, putMedia } from '$lib/idb/db';

const origin = 'https://srv.test';

function uniqueObjectUrls() {
	let n = 0;
	const created = vi.fn(() => `blob:test-${++n}`);
	const revoked = vi.fn();
	URL.createObjectURL = created;
	URL.revokeObjectURL = revoked;
	return { created, revoked };
}

describe('objectUrl', () => {
	beforeEach(() => {
		vi.resetModules();
	});

	it('returns blob URL from IDB cache without network', async () => {
		const blobId = 'blob-1';
		const data = new Uint8Array([1, 2, 3]).buffer;
		await putMedia(mediaKey(origin, blobId), { buffer: data, mime: 'image/jpeg' });

		const { getMediaUrl } = await import('./objectUrl');
		const url = await getMediaUrl(origin, blobId);
		expect(url).toMatch(/^blob:/);

		const { revokeMediaUrl } = await import('./objectUrl');
		revokeMediaUrl(origin, blobId);
	});

	// Инвариант (план 42, MED-1): живые адреса держат байты в памяти вкладки,
	// поэтому сверх бюджета отзывается давнее всех запрошенный; повторный
	// запрос «освежает» адрес, и вытесняется другой.
	it('revokes the least recently requested URL over the byte budget', async () => {
		const { revoked } = uniqueObjectUrls();
		const mod = await import('./objectUrl');
		mod.configureMediaCacheForTest({ urlBytes: 10, idbBytes: 100 });
		const four = new Uint8Array(4).buffer;
		const a = mod.seedMediaUrl(origin, 'a', four, 'image/jpeg');
		mod.seedMediaUrl(origin, 'b', four, 'image/jpeg');
		// a запрошен снова — теперь давнее всех b.
		expect(await mod.getMediaUrl(origin, 'a')).toBe(a);
		mod.seedMediaUrl(origin, 'c', four, 'image/jpeg');
		expect(revoked).toHaveBeenCalledTimes(1);
		expect(revoked).toHaveBeenCalledWith('blob:test-2');
		expect(mod.cachedMediaBytes()).toBe(8);
	});

	// Инвариант (MED-1): крупный файл из сети не копируется в IndexedDB и
	// читается как Blob, а не ArrayBuffer.
	it('does not store media above the IDB limit', async () => {
		uniqueObjectUrls();
		const body = new Uint8Array(64);
		vi.doMock('$lib/api/client', async (orig) => ({
			...(await orig<typeof import('$lib/api/client')>()),
			apiFetch: vi.fn(async () => new Response(body, { headers: { 'Content-Type': 'video/mp4' } }))
		}));
		const mod = await import('./objectUrl');
		mod.configureMediaCacheForTest({ urlBytes: 1000, idbBytes: 32 });
		const url = await mod.getMediaUrl(origin, 'big-video');
		expect(url).toMatch(/^blob:/);
		expect(await getMedia(mediaKey(origin, 'big-video'))).toBeUndefined();
		vi.doUnmock('$lib/api/client');
	});

	// Кадр ролика, снятый на устройстве, на сервере не лежит: запрос за ним
	// в сеть — это 404 на каждую плитку ленты (план 49).
	it('never asks the server for a local poster', async () => {
		uniqueObjectUrls();
		const apiFetch = vi.fn(async () => new Response(new Uint8Array(4)));
		vi.doMock('$lib/api/client', async (orig) => ({
			...(await orig<typeof import('$lib/api/client')>()),
			apiFetch
		}));
		const mod = await import('./objectUrl');
		const { localPosterId } = await import('$lib/journal/present');
		await expect(mod.getMediaUrl(origin, localPosterId('clip'))).rejects.toThrow('no_local_poster');
		expect(apiFetch).not.toHaveBeenCalled();
		const saved = mod.saveLocalPoster(origin, 'clip', new Uint8Array(4).buffer);
		expect(await mod.getMediaUrl(origin, localPosterId('clip'))).toBe(saved);
		expect(apiFetch).not.toHaveBeenCalled();
		vi.doUnmock('$lib/api/client');
	});
});

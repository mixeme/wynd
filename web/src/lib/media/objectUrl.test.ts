import { beforeEach, describe, expect, it, vi } from 'vitest';
import { mediaKey, putMedia } from '$lib/idb/db';

describe('objectUrl', () => {
	beforeEach(() => {
		vi.resetModules();
	});

	it('returns blob URL from IDB cache without network', async () => {
		const origin = 'https://srv.test';
		const blobId = 'blob-1';
		const data = new Uint8Array([1, 2, 3]).buffer;
		await putMedia(mediaKey(origin, blobId), { buffer: data, mime: 'image/jpeg' });

		const { getMediaUrl } = await import('./objectUrl');
		const url = await getMediaUrl(origin, blobId);
		expect(url).toMatch(/^blob:/);

		const { revokeMediaUrl } = await import('./objectUrl');
		revokeMediaUrl(origin, blobId);
	});
});

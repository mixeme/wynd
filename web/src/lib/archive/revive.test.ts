import { afterEach, describe, expect, it, vi } from 'vitest';

const apiJson = vi.fn();
const apiFetch = vi.fn();

vi.mock('$lib/api/client', () => ({
	apiFetch: (...args: unknown[]) => apiFetch(...args),
	apiJson: (...args: unknown[]) => apiJson(...args)
}));

vi.mock('$lib/idb/db', () => ({
	invalidateSnapshots: vi.fn()
}));

import { uploadArchiveFiles, type LoadedArchive } from './revive';

const MB = 1024 * 1024;

function archiveWith(size: number): LoadedArchive {
	const data = new Uint8Array(size);
	return {
		zip: {
			async *chunks(_entry: string, chunk: number) {
				for (let i = 0; i < size; i += chunk) yield data.subarray(i, Math.min(size, i + chunk));
			}
		},
		blobs: [{ id: 'b1', sha256: 'aa', size, mime: 'image/jpeg', filename: 'a.jpg', entry: 'media/b1.jpg' }],
		bytes: size
	} as unknown as LoadedArchive;
}

afterEach(() => {
	vi.useRealTimers();
	apiJson.mockReset();
	apiFetch.mockReset();
});

describe('uploadArchiveFiles', () => {
	it('переживает обрыв, даже когда первый вопрос «сколько принято» не прошёл', async () => {
		vi.useFakeTimers();
		apiJson.mockImplementation(async (_origin: string, path: string) => {
			if (path === '/revive/blobs') return { blobs: {} };
			if (path === '/uploads') return { id: 'u1' };
			return { id: 'blob-on-server' };
		});
		let received = 0;
		let putCalls = 0;
		let headCalls = 0;
		apiFetch.mockImplementation(async (_origin: string, _path: string, init: RequestInit) => {
			if (init.method === 'HEAD') {
				headCalls++;
				// Первый раз связи ещё нет.
				if (headCalls === 1) throw new TypeError('Failed to fetch');
				return new Response(null, { headers: { 'Upload-Offset': String(received) } });
			}
			putCalls++;
			// Второй кусок не ушёл: связь пропала.
			if (putCalls === 2) throw new TypeError('Failed to fetch');
			received += (init.body as Uint8Array).length;
			return new Response(JSON.stringify({ received_bytes: received }));
		});

		const done = uploadArchiveFiles('', archiveWith(3 * MB), () => {});
		await vi.runAllTimersAsync();
		await expect(done).resolves.toEqual({ b1: 'blob-on-server' });
		expect(received).toBe(3 * MB);
		expect(headCalls).toBe(1);
	});
});

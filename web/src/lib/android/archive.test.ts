import { beforeEach, describe, expect, it, vi } from 'vitest';

interface Port {
	postMessage(message: string): void;
	onmessage: ((event: { data: string }) => void) | null;
}

function fake(reply: (request: Record<string, unknown>) => Record<string, unknown>): {
	port: Port;
	requests: Record<string, unknown>[];
} {
	const requests: Record<string, unknown>[] = [];
	const port: Port = {
		postMessage(message: string) {
			const request = JSON.parse(message) as Record<string, unknown>;
			requests.push(request);
			queueMicrotask(() => port.onmessage?.({ data: JSON.stringify(reply(request)) }));
		},
		onmessage: null
	};
	return { port, requests };
}

describe('android archive bridge', () => {
	beforeEach(() => {
		vi.resetModules();
		delete (window as unknown as { __wyndArchive?: unknown }).__wyndArchive;
		vi.unstubAllGlobals();
	});

	it('is unavailable without the Android port', async () => {
		const { androidArchiveAvailable, androidArchiveStatus } = await import('./archive');
		expect(androidArchiveAvailable()).toBe(false);
		expect(await androidArchiveStatus()).toBeUndefined();
	});

	it('reports and opens the remembered .wynd', async () => {
		const { port, requests } = fake((request) =>
			request.op === 'open'
				? {
						id: request.id,
						archive: {
							name: 'family.wynd',
							size: 3,
							url: '/__wynd_archive/token'
						}
					}
				: { id: request.id, archive: { name: 'family.wynd', size: 3 } }
		);
		(window as unknown as { __wyndArchive?: Port }).__wyndArchive = port;
		const { androidArchiveStatus, androidArchiveFile, androidArchiveForget } =
			await import('./archive');
		expect(await androidArchiveStatus()).toEqual({ name: 'family.wynd', size: 3 });

		vi.stubGlobal(
			'fetch',
			vi.fn(async () => ({
				ok: true,
				status: 200,
				blob: async () =>
					new Blob([new Uint8Array([1, 2, 3])], { type: 'application/octet-stream' })
			}))
		);
		const restored = await androidArchiveFile();
		expect(restored?.file.name).toBe('family.wynd');
		expect(restored?.file.size).toBe(3);
		expect(restored?.size).toBe(3);

		await androidArchiveForget();
		expect(requests.map((request) => request.op)).toEqual(['status', 'open', 'forget']);
	});

	it('rejects a failed file fetch and sends forget', async () => {
		const { port, requests } = fake((request) => ({
			id: request.id,
			archive: { name: 'family.wynd', size: 3, url: '/__wynd_archive/token' }
		}));
		(window as unknown as { __wyndArchive?: Port }).__wyndArchive = port;
		vi.stubGlobal(
			'fetch',
			vi.fn(async () => ({ ok: false, status: 410, blob: async () => new Blob() }))
		);
		const { androidArchiveFile, androidArchiveForget } = await import('./archive');
		await expect(androidArchiveFile()).rejects.toThrow('Android archive: 410');
		await androidArchiveForget();
		expect(requests.map((request) => request.op)).toEqual(['open', 'forget']);
	});
});

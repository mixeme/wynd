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

describe('android native queue bridge', () => {
	beforeEach(() => {
		vi.resetModules();
		delete (window as unknown as { __wyndQueue?: unknown }).__wyndQueue;
	});

	it('is unavailable without the Android port', async () => {
		const { androidQueueAvailable, nativeEnqueue } = await import('./native-queue');
		expect(androidQueueAvailable()).toBe(false);
		expect(
			await nativeEnqueue({
				type: 'post',
				origin: 'https://srv.test',
				circle_id: 'c1',
				client_id: '1-1-1-1-1',
				payload: { body: 'x' },
				token: 't'
			})
		).toBeUndefined();
	});

	it('hands a record to the native queue', async () => {
		const { port, requests } = fake((request) => ({
			id: request.id,
			native_id: 'native-1'
		}));
		(window as unknown as { __wyndQueue?: Port }).__wyndQueue = port;
		const { androidQueueAvailable, nativeEnqueue } = await import('./native-queue');
		expect(androidQueueAvailable()).toBe(true);
		const id = await nativeEnqueue({
			type: 'post',
			origin: 'https://srv.test',
			circle_id: 'c1',
			client_id: '11111111-1111-1111-1111-111111111111',
			payload: { body: 'hello', entry_date: '2026-11-24' },
			token: 'token-1'
		});
		expect(id).toBe('native-1');
		expect(requests[0]).toMatchObject({
			op: 'enqueue',
			type: 'post',
			origin: 'https://srv.test',
			circle_id: 'c1',
			client_id: '11111111-1111-1111-1111-111111111111',
			token: 'token-1'
		});
	});

	it('reads statuses and drops malformed entries', async () => {
		const { port } = fake((request) => ({
			id: request.id,
			records: [{ client_id: 'a', state: 'pending' }],
			statuses: [
				{ client_id: 'b', state: 'sent' },
				{ client_id: 'c', state: 'unknown' },
				{ client_id: 'd', state: 'failed', error: 'invalid' }
			]
		}));
		(window as unknown as { __wyndQueue?: Port }).__wyndQueue = port;
		const { nativeStatus } = await import('./native-queue');
		expect(await nativeStatus()).toEqual({
			records: [{ client_id: 'a', state: 'pending', error: undefined }],
			statuses: [
				{ client_id: 'b', state: 'sent', error: undefined },
				{ client_id: 'd', state: 'failed', error: 'invalid' }
			]
		});
	});

	it('передаёт файлы begin → chunk → commit', async () => {
		const { port, requests } = fake((request) => {
			if (request.op === 'begin') return { id: request.id, token: 'handoff-1' };
			if (request.op === 'commit') return { id: request.id, native_id: 'native-1' };
			return { id: request.id, ok: true };
		});
		(window as unknown as { __wyndQueue?: Port }).__wyndQueue = port;
		const { nativeBegin, nativeChunk, nativeCommit } = await import('./native-queue');
		const token = await nativeBegin({
			type: 'post',
			origin: 'https://srv.test',
			circle_id: 'c1',
			client_id: '11111111-1111-1111-1111-111111111111',
			payload: {
				body: 'файл',
				entry_date: '2026-11-24',
				media_meta: [{ kind: 'photo' }]
			},
			token: 'token-1',
			files: [{ name: 'a.jpg', mime: 'image/jpeg', size: 4 }],
			aux: []
		});
		expect(token).toBe('handoff-1');
		await nativeChunk(token, 0, 0, 'AAAA');
		expect(await nativeCommit(token)).toBe('native-1');
		expect(requests.map((request) => request.op)).toEqual(['begin', 'chunk', 'commit']);
		expect(requests[0]).toMatchObject({
			files: [{ name: 'a.jpg', mime: 'image/jpeg', size: 4 }],
			aux: []
		});
	});

	it('rejects a native error and sends cancel/clear', async () => {
		const { port, requests } = fake((request) =>
			request.op === 'enqueue' ? { id: request.id, error: 'invalid' } : { id: request.id, ok: true }
		);
		(window as unknown as { __wyndQueue?: Port }).__wyndQueue = port;
		const { nativeCancel, nativeClear, nativeEnqueue } = await import('./native-queue');
		await expect(
			nativeEnqueue({
				type: 'reaction',
				origin: 'https://srv.test',
				circle_id: 'c1',
				client_id: '11111111-1111-1111-1111-111111111111',
				payload: { post_id: 'p1', emoji: '🙂' },
				token: 't'
			})
		).rejects.toThrow('Android queue: invalid');
		await nativeCancel('11111111-1111-1111-1111-111111111111');
		await nativeClear('https://srv.test');
		expect(requests.map((request) => request.op)).toEqual(['enqueue', 'cancel', 'clear']);
	});
});

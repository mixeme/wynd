import { afterEach, expect, it, vi } from 'vitest';
import { Blob as NodeBlob } from 'node:buffer';
import { saveAndroidBlob, saveAndroidDownload } from './files';

afterEach(() => {
	delete (window as unknown as { __wyndFiles?: unknown }).__wyndFiles;
});

it('оставляет обычное браузерное скачивание без возможности оболочки', async () => {
	expect(await saveAndroidBlob(new Blob(['x']), 'x.txt')).toBe(false);
	expect(await saveAndroidDownload('/archive')).toBe(false);
});

function install(cancel = false) {
	const calls: Record<string, unknown>[] = [];
	const target = {
		onmessage: null as ((event: { data: string }) => void) | null,
		postMessage(message: string) {
			const request = JSON.parse(message);
			calls.push(request);
			const state = cancel ? 'cancelled' : request.op === 'end' ? 'saved' : 'ready';
			queueMicrotask(() =>
				target.onmessage?.({ data: JSON.stringify({ id: request.id, state, detail: 'token' }) })
			);
		}
	};
	(window as unknown as { __wyndFiles: unknown }).__wyndFiles = target;
	return calls;
}

it('передаёт большой Blob частями с точным порядком и размером', async () => {
	const calls = install();
	const bytes = Uint8Array.from({ length: 65536 + 7 }, (_, i) => i % 251);
	const blob = new NodeBlob([bytes], { type: 'video/webm' }) as unknown as Blob;
	expect(await saveAndroidBlob(blob, 'test.webm')).toBe(true);
	const chunks = calls.filter((call) => call.op === 'chunk');
	expect(chunks).toHaveLength(3);
	const restored = chunks.map((chunk, index) => {
		expect(chunk.sequence).toBe(index);
		const decoded = Uint8Array.from(atob(chunk.data as string), (c) => c.charCodeAt(0));
		expect(decoded.length).toBeLessThanOrEqual(32768);
		return decoded;
	});
	expect(Buffer.concat(restored).equals(Buffer.from(bytes))).toBe(true);
	expect(calls.at(-1)).toMatchObject({ op: 'end', token: 'token', sequence: 3 });
});

it('после отмены системного диалога не читает Blob и не передаёт данные', async () => {
	const calls = install(true);
	const blob = new Blob(['private']);
	const slice = vi.spyOn(blob, 'slice');
	expect(await saveAndroidBlob(blob, 'file.txt')).toBe(true);
	expect(slice).not.toHaveBeenCalled();
	expect(calls).toHaveLength(1);
});

it('передаёт одноразовую ссылку полного потокового архива без fetch в WebView', async () => {
	const calls = install(true);
	const fetch = vi.spyOn(window, 'fetch');
	await saveAndroidDownload('/api/v1/full_archive_stream/token');
	expect(calls[0]).toMatchObject({ name: 'wynd-archive.wynd', mime: 'application/octet-stream' });
	expect(String(calls[0].url)).toContain('/api/v1/full_archive_stream/token');
	expect(fetch).not.toHaveBeenCalled();
	fetch.mockRestore();
});

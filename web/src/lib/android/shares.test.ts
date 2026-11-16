import { afterEach, expect, it, vi } from 'vitest';
import {
	appendSharedText,
	finishIncomingShare,
	loadIncomingShare,
	peekIncomingShare
} from './shares';

const token = '01234567-89ab-4cde-8123-0123456789ab';
const path = `/.wynd-android-share/${token}/0`;
function native(value: unknown, finished = true) {
	const target = {
		onmessage: null as ((event: { data: string }) => void) | null,
		postMessage(message: string) {
			const { id, op } = JSON.parse(message);
			queueMicrotask(() =>
				target.onmessage?.({
					data: JSON.stringify({
						id,
						protocol: 1,
						...(op === 'peek' ? { pending: value } : { finished })
					})
				})
			);
		}
	};
	(window as unknown as { __wyndIncomingShare: unknown }).__wyndIncomingShare = target;
}
afterEach(() => {
	delete (window as unknown as { __wyndIncomingShare?: unknown }).__wyndIncomingShare;
	vi.unstubAllGlobals();
});

it('не открывает входящий набор повторно, пока завершается отмена', async () => {
	let finishId = 0,
		peeks = 0;
	const target = {
		onmessage: null as ((event: { data: string }) => void) | null,
		postMessage(raw: string) {
			const message = JSON.parse(raw);
			if (message.op === 'finish') finishId = message.id;
			else {
				peeks++;
				queueMicrotask(() =>
					target.onmessage?.({
						data: JSON.stringify({ id: message.id, protocol: 1, pending: null })
					})
				);
			}
		}
	};
	(window as unknown as { __wyndIncomingShare: unknown }).__wyndIncomingShare = target;
	const finished = finishIncomingShare(token);
	const peek = peekIncomingShare();
	await Promise.resolve();
	expect(peeks).toBe(0);
	target.onmessage?.({ data: JSON.stringify({ id: finishId, protocol: 1, finished: true }) });
	await finished;
	expect(await peek).toBeNull();
	expect(peeks).toBe(1);
});
it('обычный браузер не имеет входящего набора', async () => {
	expect(await peekIncomingShare()).toBeNull();
});
it('не читает произвольные пути или другую capability', async () => {
	for (const bad of [
		'file:///private',
		'/api/private',
		`/.wynd-android-share/${token}/../1`,
		'https://other/file'
	]) {
		native({
			token,
			text: '',
			files: [{ name: 'photo.jpg', type: 'image/jpeg', size: 1, path: bad }]
		});
		await expect(peekIncomingShare()).rejects.toThrow('Invalid incoming media');
	}
});
it('получает текст и несколько File без подтверждения получения до чтения', async () => {
	const share = {
		token,
		text: '<b>это текст</b>\nhttps://example.org',
		files: [
			{ name: 'photo.jpg', type: 'image/jpeg', size: 3, path },
			{ name: 'clip.mp4', type: 'video/mp4', size: 2, path: `/.wynd-android-share/${token}/1` }
		]
	};
	native(share);
	const fetcher = vi
		.fn()
		.mockResolvedValueOnce({ ok: true, blob: async () => new Blob(['123']) })
		.mockResolvedValueOnce({ ok: true, blob: async () => new Blob(['45']) });
	vi.stubGlobal('fetch', fetcher);
	const received = await loadIncomingShare(token);
	expect(received.text).toBe(share.text);
	expect(received.files.map((file) => [file.name, file.type, file.size])).toEqual([
		['photo.jpg', 'image/jpeg', 3],
		['clip.mp4', 'video/mp4', 2]
	]);
	expect(fetcher).toHaveBeenCalledWith(path, {
		cache: 'no-store',
		credentials: 'omit',
		redirect: 'error'
	});
});
it('не подтверждает неполный файл или подменённый набор', async () => {
	native({ token, text: '', files: [{ name: 'photo.jpg', type: 'image/jpeg', size: 20, path }] });
	vi.stubGlobal(
		'fetch',
		vi.fn().mockResolvedValue({ ok: true, blob: async () => new Blob(['123']) })
	);
	await expect(loadIncomingShare(token)).rejects.toThrow('Incomplete');
	await expect(loadIncomingShare('ffffffff-ffff-ffff-ffff-ffffffffffff')).rejects.toThrow(
		'changed'
	);
	native(null, false);
	await expect(finishIncomingShare(token)).rejects.toThrow('changed');
});
it('добавляет текст к черновику и отказывается от переполнения без обрезания', () => {
	expect(appendSharedText('черновик', 'полученное')).toBe('черновик\n\nполученное');
	expect(appendSharedText('', 'https://example.org')).toBe('https://example.org');
	expect(() => appendSharedText('я'.repeat(16384), 'a')).toThrow('too long');
});

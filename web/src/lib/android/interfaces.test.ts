import { afterEach, describe, expect, it, vi } from 'vitest';
import {
	addInterface,
	androidInterfacesAvailable,
	loadInterfaces,
	previewInterface,
	openStartupScreen,
	removeInterface,
	selectEmbeddedInterface,
	selectInterface
} from './interfaces';

const bag = window as unknown as {
	__wyndInterfaces?: {
		postMessage: (data: string) => void;
		onmessage: ((event: { data: string }) => void) | null;
	};
};
const catalog = {
	selected: 'https://home.example.org',
	servers: [{ origin: 'https://home.example.org', name: 'Дом' }]
};
afterEach(() => {
	delete bag.__wyndInterfaces;
	vi.useRealTimers();
});
function mock(reply: Record<string, unknown>) {
	const sent: Record<string, unknown>[] = [];
	bag.__wyndInterfaces = {
		onmessage: null,
		postMessage: (raw) => {
			const value = JSON.parse(raw);
			sent.push(value);
			queueMicrotask(() =>
				bag.__wyndInterfaces?.onmessage?.({
					data: JSON.stringify({ id: value.id, protocol: 1, ...reply })
				})
			);
		}
	};
	return sent;
}
describe('Android interface source protocol', () => {
	it('is unavailable in the browser and does not call the update port', async () => {
		expect(androidInterfacesAvailable()).toBe(false);
		await expect(loadInterfaces()).rejects.toThrow('unavailable');
	});
	it('preview and add are separate requests; neither chooses the source', async () => {
		const sent = mock({ catalog, preview: { origin: 'https://files.example.org', name: 'Файлы' } });
		expect(await previewInterface('files.example.org')).toMatchObject({ name: 'Файлы' });
		expect((await addInterface('https://files.example.org')).selected).toBe(catalog.selected);
		expect(sent.map((value) => value.op)).toEqual(['preview', 'add']);
	});
	it('correlates parallel operations and passes the exact removal/selection origin', async () => {
		const sent = mock({ catalog });
		await Promise.all([
			loadInterfaces(),
			removeInterface('https://files.example.org'),
			selectInterface('https://files.example.org')
		]);
		expect(sent.map((value) => value.op)).toEqual(['list', 'remove', 'select']);
		expect(sent[1].origin).toBe('https://files.example.org');
		expect(new Set(sent.map((value) => value.id)).size).toBe(3);
	});
	it('rejects unsupported protocol, native failure and busy without success', async () => {
		mock({ protocol: 2, catalog });
		await expect(loadInterfaces()).rejects.toThrow('invalid');
		mock({ error: 'busy' });
		await expect(selectInterface('https://files.example.org')).rejects.toMatchObject({
			code: 'busy'
		});
		mock({ error: 'failed' });
		await expect(removeInterface('https://files.example.org')).rejects.toThrow('failed');
	});
	it('rejects a selected origin absent from the list and device URL in preview', async () => {
		mock({ catalog: { ...catalog, selected: 'https://missing.example.org' } });
		await expect(loadInterfaces()).rejects.toThrow('invalid');
		mock({ preview: { origin: 'file:///data/secret', name: 'Файлы' } });
		await expect(previewInterface('files.example.org')).rejects.toThrow('invalid');
	});
	it('opens the startup screen by the startup op (план 50, возврат к адресу)', async () => {
		const sent = mock({ catalog });
		await openStartupScreen();
		expect(sent).toHaveLength(1);
		expect(sent[0].op).toBe('startup');
	});
	it('embedded choice asks the embedded op and keeps the selected server', async () => {
		const sent = mock({
			catalog: { ...catalog, source: 'embedded' as const }
		});
		const reply = await selectEmbeddedInterface();
		expect(sent).toHaveLength(1);
		expect(sent[0].op).toBe('embedded');
		expect(reply.source).toBe('embedded');
		expect(reply.selected).toBe(catalog.selected);
	});
	it('server choice reports the server source of the chosen origin', async () => {
		const sent = mock({ catalog: { ...catalog, source: 'server' as const } });
		const reply = await selectInterface('https://home.example.org');
		expect(sent[0].op).toBe('select');
		expect(reply.source).toBe('server');
	});
	it('catalog without a source field is server mode (native before the switch)', async () => {
		mock({ catalog });
		const reply = await loadInterfaces();
		expect(reply.source).toBe('server');
	});
	it('foreign source value in the reply is read as server mode', async () => {
		mock({ catalog: { ...catalog, source: 'local' } });
		const reply = await loadInterfaces();
		expect(reply.source).toBe('server');
	});
	it('times out without a correlated response', async () => {
		vi.useFakeTimers();
		bag.__wyndInterfaces = { postMessage: () => {}, onmessage: null };
		const request = loadInterfaces();
		const rejected = expect(request).rejects.toThrow('timeout');
		await vi.advanceTimersByTimeAsync(25_000);
		await rejected;
	});
});

import { afterEach, expect, it, vi } from 'vitest';
import { checkInterface, interfaceAvailable, interfaceInvite, selectInterface } from './interface';

const bag = window as unknown as {
	__wyndInterface?: {
		postMessage: (message: string) => void;
		onmessage: ((event: { data: string }) => void) | null;
	};
};
afterEach(() => {
	delete bag.__wyndInterface;
	vi.useRealTimers();
});
it('keeps browser and old native wrappers free of the source capability', async () => {
	expect(interfaceAvailable()).toBe(false);
	await expect(checkInterface('home.example.org')).rejects.toThrow('unsupported');
});
it('keeps an invitation token out of the origin and refuses ambiguous navigation', () => {
	expect(interfaceInvite('home.example.org/invite/abc_DEF-1?circle=xyz')).toEqual({
		origin: 'https://home.example.org',
		path: '/invite/abc_DEF-1?circle=xyz'
	});
	for (const link of [
		'https://user:pw@home.example.org/invite/token',
		'file:///invite/token',
		'//evil.org/invite/a',
		'https://home.example.org/join/a#fragment',
		'https://home.example.org/invite/a/other',
		'/invite/a',
		'https://home.example.org/settings'
	])
		expect(interfaceInvite(link), link).toBeNull();
});
it('correlates concurrent checks and forwards only a checked ticket on selection', async () => {
	const postMessage = vi.fn();
	bag.__wyndInterface = { postMessage, onmessage: null };
	const first = checkInterface('one.example.org'),
		second = checkInterface('two.example.org');
	const sent = postMessage.mock.calls.map(([raw]) => JSON.parse(raw));
	const reply = (id: number, origin: string) =>
		bag.__wyndInterface?.onmessage?.({
			data: JSON.stringify({
				protocol: 1,
				id,
				result: { origin, name: 'Дом', allow_name_erasure: false, ticket: `ticket-${id}` }
			})
		});
	reply(sent[1].id, 'https://two.example.org/');
	reply(sent[0].id, 'https://one.example.org/');
	expect((await second).origin).toBe('https://two.example.org/');
	const checked = await first;
	const selected = selectInterface(checked.ticket, '/invite/abc');
	const request = JSON.parse(postMessage.mock.calls[2][0]);
	expect(request).toMatchObject({ method: 'select', ticket: checked.ticket, path: '/invite/abc' });
	expect(request.address).toBeUndefined();
	bag.__wyndInterface.onmessage?.({
		data: JSON.stringify({ protocol: 1, id: request.id, result: { selected: true } })
	});
	await selected;
});
it('does not treat a refusal or malformed metadata as successful selection', async () => {
	const postMessage = vi.fn();
	bag.__wyndInterface = { postMessage, onmessage: null };
	const selected = selectInterface('stale-ticket');
	let id = JSON.parse(postMessage.mock.calls[0][0]).id;
	bag.__wyndInterface.onmessage?.({ data: JSON.stringify({ protocol: 1, id, error: 'expired' }) });
	await expect(selected).rejects.toThrow('expired');
	const check = checkInterface('home.example.org');
	id = JSON.parse(postMessage.mock.calls[1][0]).id;
	bag.__wyndInterface.onmessage?.({
		data: JSON.stringify({ protocol: 1, id, result: { origin: 'https://home.example.org' } })
	});
	await expect(check).rejects.toThrow('invalid');
});
it('times out if a foreign protocol answers', async () => {
	vi.useFakeTimers();
	const postMessage = vi.fn();
	bag.__wyndInterface = { postMessage, onmessage: null };
	const check = checkInterface('home.example.org');
	const rejection = expect(check).rejects.toThrow('timeout');
	const id = JSON.parse(postMessage.mock.calls[0][0]).id;
	bag.__wyndInterface.onmessage?.({ data: JSON.stringify({ protocol: 99, id, result: {} }) });
	await vi.advanceTimersByTimeAsync(25000);
	await rejection;
});

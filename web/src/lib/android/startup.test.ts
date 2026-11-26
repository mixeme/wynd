import { afterEach, expect, it, vi } from 'vitest';
import { startupAvailable, startupRequest } from './startup';

const bag = window as unknown as {
	__wyndStartup?: {
		postMessage(data: string): void;
		onmessage: ((event: { data: string }) => void) | null;
	};
};
afterEach(() => {
	delete bag.__wyndStartup;
	vi.useRealTimers();
});
function mock() {
	const sent: Array<{ id: number; op: string; address?: string }> = [];
	const target = (bag.__wyndStartup = {
		onmessage: null as ((event: { data: string }) => void) | null,
		postMessage: (raw: string) => {
			sent.push(JSON.parse(raw));
		}
	});
	return {
		sent,
		reply(index: number, value: object) {
			target.onmessage?.({ data: JSON.stringify({ id: sent[index].id, protocol: 1, ...value }) });
		}
	};
}
it('does not expose startup operations in the server page or browser', async () => {
	expect(startupAvailable()).toBe(false);
	await expect(startupRequest('state')).rejects.toThrow('unavailable');
});
it('keeps invitation case and query in a check, and never selects automatically', async () => {
	const mockPort = mock();
	const address = 'https://home.example.org/invite/AbC-7?next=Join';
	const checked = startupRequest('check', address);
	expect(mockPort.sent).toEqual([{ id: expect.any(Number), op: 'check', address }]);
	mockPort.reply(0, { preview: { name: 'Дом', origin: 'https://home.example.org' } });
	expect((await checked).preview?.name).toBe('Дом');
	expect(mockPort.sent).toHaveLength(1);
	const opened = startupRequest('open', address);
	expect(mockPort.sent[1]).toMatchObject({ op: 'open', address });
	mockPort.reply(1, {});
	await opened;
});
it('correlates cancellation and late check replies without treating them as one result', async () => {
	const mockPort = mock();
	const checked = startupRequest('check', 'old.example.org');
	const cancelled = startupRequest('cancel');
	mockPort.reply(1, {});
	await cancelled;
	mockPort.reply(0, { error: 'failed' });
	await expect(checked).rejects.toThrow('failed');
});
it('rejects incompatible protocol and failed writes rather than confirming selection', async () => {
	const mockPort = mock();
	const opened = startupRequest('open', 'home.example.org');
	mockPort.reply(0, { error: 'failed' });
	await expect(opened).rejects.toThrow('failed');
	const state = startupRequest('state');
	mockPort.reply(1, { protocol: 2, offline: false });
	await expect(state).rejects.toThrow('failed');
});
it('times out missing replies and tolerates malformed messages', async () => {
	vi.useFakeTimers();
	mock();
	const request = startupRequest('retry');
	const rejected = expect(request).rejects.toThrow('timeout');
	bag.__wyndStartup!.onmessage?.({ data: '{broken' });
	await vi.advanceTimersByTimeAsync(30000);
	await rejected;
});
it('passes the login email only with open (план 72)', async () => {
	const mockPort = mock();
	const checked = startupRequest('check', 'home.example.org');
	expect(mockPort.sent[0]).not.toHaveProperty('email');
	mockPort.reply(0, { preview: { name: 'Дом', origin: 'https://home.example.org' } });
	await checked;
	const opened = startupRequest('open', 'home.example.org', 'anya@example.com');
	expect(mockPort.sent[1]).toMatchObject({
		op: 'open',
		address: 'home.example.org',
		email: 'anya@example.com'
	});
	mockPort.reply(1, {});
	await opened;
});

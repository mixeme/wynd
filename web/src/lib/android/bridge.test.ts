import { afterEach, describe, expect, it, vi } from 'vitest';
import { androidPort, androidRequest } from './bridge';

const bag = window as unknown as {
	__wyndAndroid?: {
		postMessage: (message: string) => void;
		onmessage: ((event: { data: string }) => void) | null;
	};
};
afterEach(() => {
	delete bag.__wyndAndroid;
	vi.useRealTimers();
});

describe('Android capability requests', () => {
	it('does not advertise Android in a normal browser', async () => {
		expect(androidPort()).toBeUndefined();
		await expect(androidRequest('status')).rejects.toThrow('unavailable');
	});
	it('correlates replies and ignores malformed or unknown responses', async () => {
		const postMessage = vi.fn();
		bag.__wyndAndroid = { postMessage, onmessage: null };
		const request = androidRequest('setImmediately', true);
		const sent = JSON.parse(postMessage.mock.calls[0][0]);
		expect(sent).toMatchObject({ method: 'setImmediately', value: true });
		bag.__wyndAndroid.onmessage?.({ data: '{' });
		bag.__wyndAndroid.onmessage?.({
			data: JSON.stringify({ id: sent.id + 1, result: { version: '9.9.9' } })
		});
		bag.__wyndAndroid.onmessage?.({
			data: JSON.stringify({ id: sent.id, result: { version: '0.1.0', immediately: true } })
		});
		await expect(request).resolves.toMatchObject({ version: '0.1.0', immediately: true });
	});
	it('times out without accepting a response for a different request', async () => {
		vi.useFakeTimers();
		bag.__wyndAndroid = { postMessage: vi.fn(), onmessage: null };
		const request = androidRequest('check');
		const rejection = expect(request).rejects.toThrow('timed out');
		await vi.advanceTimersByTimeAsync(180_000);
		await rejection;
	});
});

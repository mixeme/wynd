import { afterEach, describe, expect, it, vi } from 'vitest';
import { resolveServerOrigin } from './origin';

describe('resolveServerOrigin', () => {
	const origin = 'http://127.0.0.1:5173';

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('treats loopback on another port as a distinct server', () => {
		vi.stubGlobal('window', { location: { origin, hostname: '127.0.0.1', host: '127.0.0.1:5173' } });
		expect(resolveServerOrigin('http://127.0.0.1:19999')).toBe('http://127.0.0.1:19999');
	});

	it('maps exact page origin to same-host proxy', () => {
		vi.stubGlobal('window', { location: { origin, hostname: '127.0.0.1', host: '127.0.0.1:5173' } });
		expect(resolveServerOrigin('http://127.0.0.1:5173')).toBe('');
	});
});

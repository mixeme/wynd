import 'fake-indexeddb/auto';
import { afterEach, beforeEach, vi } from 'vitest';
import { closeDb } from '$lib/idb/db';

function mockMatchMedia(matches = false) {
	return {
		matches,
		media: '',
		onchange: null,
		addEventListener: vi.fn(),
		removeEventListener: vi.fn(),
		addListener: vi.fn(),
		removeListener: vi.fn(),
		dispatchEvent: vi.fn()
	};
}

beforeEach(() => {
	vi.stubGlobal('navigator', { ...navigator, onLine: false });
	const media = mockMatchMedia();
	vi.stubGlobal('matchMedia', vi.fn(() => media));
	Object.defineProperty(window, 'matchMedia', {
		writable: true,
		value: vi.fn(() => media)
	});
	if (!URL.createObjectURL) {
		URL.createObjectURL = vi.fn(() => 'blob:mock');
	}
	if (!URL.revokeObjectURL) {
		URL.revokeObjectURL = vi.fn();
	}
});

afterEach(async () => {
	await closeDb();
	vi.unstubAllGlobals();
	vi.resetModules();
});

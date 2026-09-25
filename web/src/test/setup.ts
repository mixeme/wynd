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

function installMatchMedia() {
	const media = mockMatchMedia();
	const fn = vi.fn(() => media);
	vi.stubGlobal('matchMedia', fn);
	Object.defineProperty(window, 'matchMedia', {
		writable: true,
		configurable: true,
		value: fn
	});
}

installMatchMedia();

beforeEach(() => {
	vi.stubGlobal('navigator', { ...navigator, onLine: false });
	installMatchMedia();
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
	installMatchMedia();
	vi.resetModules();
});

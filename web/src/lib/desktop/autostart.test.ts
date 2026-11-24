import { afterEach, describe, expect, it, vi } from 'vitest';
import { autostartBridge } from './autostart';

const bag = window as unknown as {
	__wyndDesktop?: { origin: string };
	go?: { main?: { App?: Record<string, unknown> } };
};

afterEach(() => {
	delete bag.__wyndDesktop;
	delete bag.go;
	vi.restoreAllMocks();
});

describe('autostart bridge', () => {
	it('без моста оболочки молчит', () => {
		expect(autostartBridge()).toBeUndefined();
	});

	it('зовёт методы окна', async () => {
		bag.__wyndDesktop = { origin: 'https://home.example.org' };
		const state = { enabled: false, stale: false, command: '' };
		const get = vi.fn().mockResolvedValue(state);
		const set = vi.fn().mockResolvedValue({ ...state, enabled: true });
		bag.go = { main: { App: { Autostart: get, SetAutostart: set } } };
		const bridge = autostartBridge();
		expect(bridge).toBeDefined();
		await expect(bridge!.get()).resolves.toEqual(state);
		await expect(bridge!.set(true)).resolves.toMatchObject({ enabled: true });
		expect(set).toHaveBeenCalledWith(true);
	});
});

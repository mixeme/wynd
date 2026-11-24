import { afterEach, describe, expect, it, vi } from 'vitest';
import { autostartBridge, autostartSystemName } from './autostart';

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

describe('autostart system name', () => {
	it('на Windows оставляет привычный текст', () => {
		expect(autostartSystemName('Mozilla/5.0 (Windows NT 10.0; Win64; x64)')).toBe('Windows');
	});

	it('на Linux не говорит про Windows', () => {
		expect(autostartSystemName('Mozilla/5.0 (X11; Ubuntu; Linux x86_64)')).toBe('систему');
	});
});

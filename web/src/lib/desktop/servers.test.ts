import { afterEach, describe, expect, it, vi } from 'vitest';
import { allowDesktopServer, openDesktopInterface, registerDesktopServer } from './servers';

const bag = window as unknown as {
	__wyndDesktop?: { origin: string };
	go?: { main?: { App?: Record<string, unknown> } };
};

afterEach(() => {
	delete bag.__wyndDesktop;
	delete bag.go;
	vi.restoreAllMocks();
});

describe('desktop server registration', () => {
	it('без моста ничего не делает', async () => {
		await expect(allowDesktopServer('https://other.example.org')).resolves.toBeUndefined();
		await expect(
			registerDesktopServer('https://other.example.org', 'У Славы')
		).resolves.toBeUndefined();
	});

	it('разрешает адрес до проверки и сохраняет имя после входа', async () => {
		bag.__wyndDesktop = { origin: 'https://home.example.org' };
		const allow = vi.fn().mockResolvedValue('https://other.example.org');
		const add = vi.fn().mockResolvedValue(undefined);
		bag.go = { main: { App: { AllowServer: allow, AddServer: add } } };
		await allowDesktopServer('https://other.example.org');
		await registerDesktopServer('https://other.example.org', 'У Славы', '0.50.0', 'invite');
		expect(allow).toHaveBeenCalledWith('https://other.example.org');
		expect(add).toHaveBeenCalledWith('https://other.example.org', 'У Славы', '0.50.0', 'invite');
	});

	it('просит оболочку открыть источник интерфейса', () => {
		bag.__wyndDesktop = { origin: 'https://home.example.org' };
		const post = vi.spyOn(window.parent, 'postMessage').mockImplementation(() => {});
		openDesktopInterface();
		expect(post).toHaveBeenCalledWith({ type: 'wynd:open-interface' }, window.location.origin);
	});
});

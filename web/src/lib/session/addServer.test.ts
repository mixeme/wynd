import { afterEach, describe, expect, it, vi } from 'vitest';

type Bag = { __wyndDesktop?: unknown; __wyndAndroid?: unknown };
const bag = window as unknown as Bag;

afterEach(() => {
	delete bag.__wyndDesktop;
	delete bag.__wyndAndroid;
	vi.resetModules();
});

async function flag(): Promise<boolean> {
	vi.resetModules();
	return (await import('./addServer')).ADD_SERVER_IN_APP;
}

describe('ADD_SERVER_IN_APP', () => {
	it('в браузере без мостов — false', async () => {
		expect(await flag()).toBe(false);
	});

	it('в окне для компьютера — true', async () => {
		bag.__wyndDesktop = { origin: 'https://home.example' };
		expect(await flag()).toBe(true);
	});

	it('в Android — true', async () => {
		bag.__wyndAndroid = { postMessage: () => {} };
		expect(await flag()).toBe(true);
	});

	it('мост Android без postMessage не считается', async () => {
		bag.__wyndAndroid = {};
		expect(await flag()).toBe(false);
	});
});

import { beforeEach, describe, expect, it, vi } from 'vitest';
import { saveAppSettings } from '$lib/idb/db';

describe('session', () => {
	beforeEach(() => {
		vi.resetModules();
	});

	it('loads theme from IDB on init', async () => {
		await saveAppSettings({ theme: 'dark' });
		const session = await import('./session.svelte');
		await session.initSession();
		expect(session.getTheme()).toBe('dark');
		expect(session.isDark()).toBe(true);
	});

	it('persists theme changes', async () => {
		const { getAppSettings } = await import('$lib/idb/db');
		const session = await import('./session.svelte');
		await session.initSession();
		session.setTheme('light');
		expect(session.getTheme()).toBe('light');
		await vi.waitFor(async () => {
			expect((await getAppSettings())?.theme).toBe('light');
		});
	});

	it('stores and loads participant sessions', async () => {
		const session = await import('./session.svelte');
		const record = {
			origin: 'https://a.test',
			name: 'Alice',
			email: 'a@test',
			token: 'tok',
			account_id: 'acc1'
		};
		await session.storeSession(record);
		const loaded = await session.loadSession('https://a.test');
		expect(loaded).toEqual(record);
		expect(await session.loadSessions()).toHaveLength(1);
	});

	it('stores admin session separately', async () => {
		const session = await import('./session.svelte');
		await session.storeAdminSession({ origin: '', token: 'admin-tok' });
		expect(await session.loadAdminSession()).toEqual({ origin: '', token: 'admin-tok' });
		await session.removeAdminSession();
		expect(await session.loadAdminSession()).toBeUndefined();
	});
});

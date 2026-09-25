import { beforeEach, describe, expect, it, vi } from 'vitest';
import { ApiError } from '$lib/api/client';
import { saveAppSettings, putSession, putSnapshot, snapshotKey, getSession } from '$lib/idb/db';

vi.mock('$lib/api/client', async (importOriginal) => {
	const actual = await importOriginal<typeof import('$lib/api/client')>();
	return {
		...actual,
		apiJson: vi.fn()
	};
});

describe('session', () => {
	beforeEach(async () => {
		vi.resetModules();
		vi.clearAllMocks();
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

	it('detects rejected bearer tokens', async () => {
		const session = await import('./session.svelte');
		expect(session.isSessionRejected(new ApiError(403, 'forbidden'))).toBe(true);
		expect(session.isSessionRejected(new ApiError(401, 'forbidden'))).toBe(true);
		expect(session.isSessionRejected(new ApiError(404, 'not_found'))).toBe(false);
	});

	it('drops stale participant sessions during reconcile', async () => {
		const { apiJson } = await import('$lib/api/client');
		vi.mocked(apiJson).mockRejectedValue(new ApiError(403, 'forbidden'));
		await putSession({
			origin: '',
			name: 'Dev',
			email: 'a@test',
			token: 'stale',
			account_id: 'acc1'
		});
		await putSnapshot(snapshotKey('', 'circles', '_list'), {
			data: { circles: [{ id: 'c1', name: 'Old', status: 'active' }] },
			fetched_at: Date.now()
		});

		const session = await import('./session.svelte');
		await session.reconcileStoredSessions();

		expect(await getSession('')).toBeUndefined();
		expect(await session.loadSessions()).toHaveLength(0);
	});
});

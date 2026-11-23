import 'fake-indexeddb/auto';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { getDb, putSession, putSnapshot, snapshotKey } from '$lib/idb/db';
import { SETTINGS_BUNDLE_VERSION, loadSettingsBundle, loadSettingsBundleCached } from './settings';

// C125: экран настроек круга рисует первый кадр одним снимком. Квота («Место»)
// и подпись уведомлений раньше приходили отдельными ответами уже после первого
// кадра: экран становился выше, и возвращённая прокрутка уезжала. Снимок должен
// нести всё, что видно на экране.
const ORIGIN = '';
const CIRCLE = 'c1';

const SETTINGS = {
	id: CIRCLE,
	name: 'Дача',
	status: 'active',
	edit_window_sec: 3600,
	is_owner: true,
	can_settings: true,
	identity_id: 'i1',
	identity_name: 'Аня',
	invite_who: 'all',
	invite_kind_default: 'single',
	share_place: false
};

const MEMBERS = {
	members: [
		{
			identity_id: 'i1',
			name: 'Аня',
			status: 'active',
			can_settings: true,
			is_owner: true,
			joined_at: '2026-11-01T00:00:00Z',
			can_read: true,
			can_write: true
		},
		{
			identity_id: 'i2',
			name: 'Боб',
			status: 'active',
			can_settings: false,
			is_owner: false,
			joined_at: '2026-11-01T00:00:00Z',
			can_read: true,
			can_write: true
		}
	]
};

const QUOTA = { used_bytes: 1024, post_count: 1, quota_bytes: 2048, volume: [] };
const NOTIFY = {
	posts: true,
	comments_mine: true,
	comments_all: false,
	reactions: true,
	mentions: true,
	events: false,
	mute_until: null
};

async function clear(): Promise<void> {
	const db = await getDb();
	const stores = ['sessions', 'snapshots', 'settings', 'groups', 'pins', 'street_ops'] as const;
	for (const store of stores) {
		await db.clear(store);
	}
}

function mockFetch(settings: unknown = SETTINGS, overrides: Record<string, unknown> = {}) {
	const calls: string[] = [];
	const bodies: Record<string, unknown> = {
		[`/api/v1/circles/${CIRCLE}`]: settings,
		[`/api/v1/circles/${CIRCLE}/members`]: MEMBERS,
		[`/api/v1/circles/${CIRCLE}/notify_prefs`]: NOTIFY,
		...overrides
	};
	const spy = vi.spyOn(globalThis, 'fetch').mockImplementation((async (input: unknown) => {
		const url = String(input);
		calls.push(url);
		const key = Object.keys(bodies).find((k) => url === k || url.startsWith(k + '?'));
		if (!key) return new Response(JSON.stringify({ error: 'not_found' }), { status: 404 });
		return new Response(JSON.stringify(bodies[key]), { status: 200 });
	}) as typeof fetch);
	return { spy, calls };
}

beforeEach(async () => {
	await clear();
	await putSession({
		origin: ORIGIN,
		name: 'Стенд',
		email: 'anya@example.com',
		token: 't',
		account_id: 'a'
	});
});

describe('снимок экрана настроек (C125)', () => {
	it('владельцу — настройки, участники, квота и уведомления одним снимком', async () => {
		const { calls } = mockFetch(SETTINGS, { [`/api/v1/circles/${CIRCLE}/quota`]: QUOTA });

		const bundle = await loadSettingsBundle(ORIGIN, CIRCLE);

		expect(bundle.v).toBe(SETTINGS_BUNDLE_VERSION);
		expect(bundle.quota?.used_bytes).toBe(1024);
		expect(bundle.notify?.posts).toBe(true);
		expect(calls.some((url) => url.startsWith(`/api/v1/circles/${CIRCLE}/quota`))).toBe(true);
		expect(calls.some((url) => url === `/api/v1/circles/${CIRCLE}/notify_prefs`)).toBe(true);

		const cached = await loadSettingsBundleCached(ORIGIN, CIRCLE);
		expect(cached?.v).toBe(SETTINGS_BUNDLE_VERSION);
		expect(cached?.quota?.used_bytes).toBe(1024);
		expect(cached?.notify?.posts).toBe(true);
	});

	it('не владельцу квоту не просит: сервер её не отдаёт', async () => {
		const { calls } = mockFetch({ ...SETTINGS, is_owner: false, can_settings: false });

		const bundle = await loadSettingsBundle(ORIGIN, CIRCLE);

		expect(bundle.quota).toBeUndefined();
		expect(bundle.notify?.posts).toBe(true);
		expect(calls.some((url) => url.startsWith(`/api/v1/circles/${CIRCLE}/quota`))).toBe(false);
	});

	it('снимок прежнего формата у владельца не рисуется: он короче экрана', async () => {
		await putSnapshot(snapshotKey(ORIGIN, 'settings', CIRCLE), {
			data: { settings: SETTINGS, members: MEMBERS.members },
			fetched_at: Date.now()
		});
		expect(await loadSettingsBundleCached(ORIGIN, CIRCLE)).toBeUndefined();

		await putSnapshot(snapshotKey(ORIGIN, 'settings', CIRCLE), {
			data: { settings: SETTINGS, members: MEMBERS.members, quota: QUOTA, notify: NOTIFY, v: 2 },
			fetched_at: Date.now()
		});
		expect((await loadSettingsBundleCached(ORIGIN, CIRCLE))?.quota?.used_bytes).toBe(1024);
	});
});

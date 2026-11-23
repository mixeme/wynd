import 'fake-indexeddb/auto';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount, type Component } from 'svelte';
import { getDb, putSnapshot, snapshotKey } from '$lib/idb/db';
import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
import {
	createCircleInvite,
	fetchCircleSettings,
	fetchInviteCandidates,
	fetchMembers,
	fetchNotifyPrefs
} from '$lib/circles/settings';
import InvitePage from '../../routes/circles/[id]/settings/invite/+page.svelte';
import NotifyPage from '../../routes/circles/[id]/settings/notify/+page.svelte';

// C125: первый кадр = конечный экран по высоте. Блок, который может прийти
// позже и встать над содержимым, либо уже в первом кадре (снимок), либо экран
// ждёт ответ. «Форма сразу, блок сверху потом» не делаем.
vi.mock('$app/navigation', () => ({
	goto: vi.fn(async () => {}),
	beforeNavigate: vi.fn(),
	afterNavigate: vi.fn()
}));
vi.mock('$app/stores', async () => {
	const { readable } = await import('svelte/store');
	return { page: readable({ url: new URL('http://localhost/circles/c1/settings/invite') }) };
});
vi.mock('$lib/navigation/up', () => ({
	navigateBack: vi.fn(),
	goUp: vi.fn()
}));
vi.mock('$lib/auth/auth', () => ({ authErrorHint: () => 'Не получилось' }));
vi.mock('$lib/clipboard', () => ({ copyText: vi.fn(async () => {}) }));
vi.mock('$lib/circles/settings', async (importOriginal) => {
	const real = await importOriginal<typeof import('$lib/circles/settings')>();
	return {
		...real,
		createCircleInvite: vi.fn(),
		fetchCircleSettings: vi.fn(),
		fetchInviteCandidates: vi.fn(),
		fetchNotifyPrefs: vi.fn(),
		fetchMembers: vi.fn(),
		saveNotifyPrefs: vi.fn()
	};
});

const CIRCLE = {
	origin: '',
	circleId: 'c1',
	name: 'Семья',
	color: 'ochre',
	colorHex: 'var(--ochre)',
	identityName: 'Аня',
	identityId: 'i1',
	identityInitial: 'А',
	avatarBlobId: '',
	avatarUrl: '',
	editWindowSec: 3600,
	lastReadSeq: 0,
	canWrite: true,
	spaceFull: '',
	ownerName: '',
	youOwn: true,
	frozenAt: '',
	serverName: '',
	sharePlace: false,
	responsesUnread: 0,
	hasOthers: true,
	refresh: async () => {}
} as unknown as CircleContext;

const PREFS = {
	posts: true,
	comments_mine: true,
	comments_all: false,
	reactions: false,
	events: false,
	mute_until: null
};

function deferred<T>() {
	let resolve!: (value: T) => void;
	const promise = new Promise<T>((r) => (resolve = r));
	return { promise, resolve };
}

let close: (() => Promise<void>) | undefined;
let target: HTMLElement;

async function show(page: Component) {
	await close?.();
	target = document.createElement('div');
	document.body.append(target);
	const component = mount(page, { target, context: new Map([[CIRCLE_CTX, CIRCLE]]) });
	flushSync();
	close = async () => {
		await unmount(component);
		target.remove();
	};
	return target;
}

beforeEach(async () => {
	const db = await getDb();
	const stores = ['sessions', 'snapshots', 'settings', 'groups', 'pins', 'street_ops'] as const;
	for (const store of stores) await db.clear(store);
	vi.mocked(fetchCircleSettings).mockReset();
	vi.mocked(fetchInviteCandidates).mockReset();
	vi.mocked(createCircleInvite).mockReset();
});

afterEach(async () => {
	await close?.();
	close = undefined;
});

describe('первый кадр без прыжков (C125)', () => {
	it('«Пригласить»: до ссылки — загрузка, без чипов и карточки', async () => {
		vi.mocked(fetchCircleSettings).mockResolvedValue({
			is_owner: true,
			can_settings: true,
			invite_kind_default: 'single'
		} as never);
		vi.mocked(fetchInviteCandidates).mockResolvedValue([] as never);
		const link = deferred<{ id: string; token: string }>();
		vi.mocked(createCircleInvite).mockReturnValue(link.promise as never);

		const screen = await show(InvitePage);

		// Ссылка ещё создаётся: экран не показывает ни чипов, ни карточки — они
		// встали бы над подсказкой и сдвинули всё, что ниже.
		expect(screen.textContent).not.toContain('Одноразовая');
		expect(screen.textContent).not.toContain('Скопировать');
		expect(screen.textContent).toContain('Загрузка');

		link.resolve({ id: 'inv1', token: 'tok' });
		await vi.waitFor(() => {
			flushSync();
			expect(screen.textContent).toContain('Одноразовая');
		});
		expect(screen.textContent).toContain('Скопировать');
	});

	it('«Уведомления»: строки из снимка, не дожидаясь сети', async () => {
		// Снимок семейства настроек: круг из одного участника, prefs записаны.
		await putSnapshot(snapshotKey('', 'settings', 'c1'), {
			data: {
				settings: { id: 'c1', name: 'Семья', status: 'active', is_owner: true, can_settings: true },
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
					}
				],
				notify: PREFS,
				quota: { used_bytes: 1, quota_bytes: undefined },
				v: 2
			},
			fetched_at: Date.now()
		});
		// Сеть не отвечает: первый кадр должен быть полным без неё.
		const networkPrefs = deferred<typeof PREFS>();
		vi.mocked(fetchNotifyPrefs).mockReturnValue(networkPrefs.promise as never);
		vi.mocked(fetchMembers).mockReturnValue(new Promise(() => {}) as never);

		const screen = await show(NotifyPage);
		await vi.waitFor(() => {
			flushSync();
			expect(screen.textContent).toContain('Комментарии к моим записям');
		});
		// Круг из одного: строк «Новые записи», «Все комментарии», «Реакции» нет
		// с первого кадра — раньше они появлялись и исчезали через 0,2 с.
		expect(screen.textContent).not.toContain('Новые записи');
		expect(screen.textContent).not.toContain('Все комментарии');
		expect(screen.textContent).not.toContain('Реакции');

		// Сеть ответила тем же — состав не меняется.
		networkPrefs.resolve(PREFS);
		await vi.waitFor(() => {
			flushSync();
			expect(screen.textContent).not.toContain('Новые записи');
		});
	});
});

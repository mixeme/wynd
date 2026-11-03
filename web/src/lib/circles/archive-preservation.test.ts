import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount, type Component } from 'svelte';
import FullPage from '../../routes/circles/[id]/quota/full/+page.svelte';
import MembersPage from '../../routes/circles/[id]/settings/members/+page.svelte';
import { apiJson } from '$lib/api/client';
import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
import { PARTIAL_ARCHIVE_WARNING, type FullArchiveState } from './fullarchive';
import { transferOwnership } from './settings';

vi.mock('$app/navigation', () => ({
	goto: vi.fn(),
	beforeNavigate: vi.fn(),
	afterNavigate: vi.fn()
}));
vi.mock('$app/stores', async () => {
	const { readable } = await import('svelte/store');
	return {
		page: readable({ url: new URL('http://localhost/circles/c1/settings/members?transfer=1') })
	};
});
vi.mock('$lib/navigation/up', () => ({ goUp: vi.fn(), setBackHandler: () => () => {} }));
vi.mock('$lib/api/client', async (original) => ({
	...(await original<typeof import('$lib/api/client')>()),
	apiJson: vi.fn()
}));
vi.mock('$lib/idb/db', async (original) => ({
	...(await original<typeof import('$lib/idb/db')>()),
	getSession: vi.fn().mockResolvedValue({ account_id: 'owner' }),
	invalidateSnapshots: vi.fn()
}));

let dispose: (() => Promise<void>) | undefined;
beforeEach(() => vi.mocked(apiJson).mockReset());
afterEach(async () => {
	await dispose?.();
	dispose = undefined;
});
function openPage(page: Component) {
	const target = document.createElement('div');
	document.body.append(target);
	const circle = {
		origin: '',
		circleId: 'c1',
		name: 'Семья',
		color: 'terracotta',
		identityId: 'i1',
		refresh: vi.fn()
	} as unknown as CircleContext;
	const instance = mount(page, { target, context: new Map([[CIRCLE_CTX, circle]]) });
	dispose = async () => {
		await unmount(instance);
		target.remove();
	};
	return target;
}
function button(text: string, root: ParentNode = document) {
	const found = [...root.querySelectorAll('button')].find((b) => b.textContent?.trim() === text);
	expect(found, text).toBeDefined();
	return found!;
}

// Выбор относится к конкретной передаче; закрытие не сохраняет прежнее согласие.
it('передаёт прошлое только после явного выбора и сбрасывает его при отмене', async () => {
	const base = {
		status: 'active',
		joined_at: '2026-11-01T12:00:00Z',
		can_read: true,
		can_write: true,
		can_settings: true
	};
	vi.mocked(apiJson).mockResolvedValue({
		members: [
			{ ...base, account_id: 'owner', identity_id: 'i1', name: 'Аня', is_owner: true },
			{ ...base, account_id: 'new', identity_id: 'i2', name: 'Боб', is_owner: false }
		]
	});
	const target = openPage(MembersPage);
	await vi.waitFor(() => expect(target.textContent).toContain('Боб'));
	const pick = () => {
		[...target.querySelectorAll('button')].find((b) => b.textContent?.includes('Боб'))!.click();
		flushSync();
	};
	pick();
	const toggle = () => document.querySelector<HTMLButtonElement>('[role="switch"]')!;
	expect(toggle().getAttribute('aria-checked')).toBe('false');
	toggle().click();
	flushSync();
	expect(document.body.textContent).toContain(
		'В архив нового владельца войдёт и переданное вам для сохранения прошлое.'
	);
	button('Отмена').click();
	flushSync();
	pick();
	expect(toggle().getAttribute('aria-checked')).toBe('false');
	toggle().click();
	flushSync();
	let finish!: () => void;
	vi.mocked(apiJson).mockResolvedValueOnce({ archive_preservation: true });
	vi.mocked(apiJson).mockReturnValueOnce(
		new Promise((resolve) => {
			finish = () => resolve({});
		})
	);
	button('Передать').click();
	flushSync();
	expect(toggle().disabled).toBe(true);
	await vi.waitFor(() => expect(vi.mocked(apiJson)).toHaveBeenCalledTimes(3));
	expect(JSON.parse(vi.mocked(apiJson).mock.calls.at(-1)![2]!.body as string)).toEqual({
		new_owner_account_id: 'new',
		transfer_past: true
	});
	finish();
	await Promise.resolve();
});

it('не передаёт владение старому серверу, который проигнорирует выбор прошлого', async () => {
	vi.mocked(apiJson).mockResolvedValue({ version: '0.45.1' });
	await expect(transferOwnership('', 'c1', 'new')).rejects.toMatchObject({
		code: 'archive_preservation_unsupported'
	});
	expect(vi.mocked(apiJson)).toHaveBeenCalledTimes(1);
	expect(vi.mocked(apiJson).mock.calls[0][1]).toBe('/instance');
});

const downloaded: FullArchiveState = {
	estimate_bytes: 1000,
	posts: 99,
	members: 2,
	fits: true,
	retention_hours: 72,
	full_journal: true,
	frozen_at: '2026-11-01T12:00:00Z',
	archive: {
		id: 'file',
		status: 'removed',
		requested_at: '2026-11-01T12:00:00Z',
		snapshot_at: '2026-11-01T12:00:00Z',
		downloaded_at: '2026-11-01T12:01:00Z',
		removed_reason: 'downloaded',
		delete_circle: true,
		encrypted: false,
		size_bytes: 1000,
		full_journal: false,
		posts: 3
	}
};

// Старый файл описывается собственным снимком, а не новым составом круга.
it('предупреждает о неполной копии и передаёт согласие только из подтверждения', async () => {
	vi.mocked(apiJson).mockResolvedValue(downloaded);
	const target = openPage(FullPage);
	await vi.waitFor(() => expect(target.textContent).toContain(PARTIAL_ARCHIVE_WARNING));
	expect(target.textContent).toContain('Часть журнала · 3 записи');
	button('Убрать круг с сервера', target).click();
	flushSync();
	expect(vi.mocked(apiJson)).toHaveBeenCalledTimes(1);
	expect(document.body.textContent!.split(PARTIAL_ARCHIVE_WARNING)).toHaveLength(3);
	button('Убрать').click();
	await vi.waitFor(() => expect(vi.mocked(apiJson)).toHaveBeenCalledTimes(2));
	expect(JSON.parse(vi.mocked(apiJson).mock.calls[1][2]!.body as string)).toEqual({
		acknowledge_partial: true
	});
});

it('до сборки показывает разрешённую часть журнала и её число записей', async () => {
	vi.mocked(apiJson).mockResolvedValue({
		...downloaded,
		full_journal: false,
		posts: 3,
		archive: null,
		frozen_at: null
	});
	const target = openPage(FullPage);
	await vi.waitFor(() => expect(target.textContent).toContain('Часть журнала «Семья»'));
	expect(target.textContent).toContain('3 записи');
});

import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import type { Writable } from 'svelte/store';
import { goto } from '$app/navigation';
import { ApiError, apiJson } from '$lib/api/client';
import { openDownload } from '$lib/navigation/openDownload';
import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
import ArchivePage from '../../routes/circles/[id]/archive/+page.svelte';
import BusyPage from '../../routes/archive/busy/+page.svelte';
import { archiveDownloadState, startArchiveDownload } from './archive-download.svelte';

const testPage = vi.hoisted(() => ({ store: undefined as unknown }));
vi.mock('$app/stores', async () => {
	const { writable } = await import('svelte/store');
	const page = writable({ url: new URL('http://localhost/archive/busy?retry_after=30') });
	testPage.store = page;
	return { page };
});
vi.mock('$app/navigation', () => ({
	goto: vi.fn(),
	beforeNavigate: vi.fn(),
	afterNavigate: vi.fn()
}));
vi.mock('$lib/navigation/up', () => ({ goUp: vi.fn(), setBackHandler: () => () => {} }));
vi.mock('$lib/navigation/openDownload', () => ({ openDownload: vi.fn() }));
vi.mock('$lib/api/client', async (original) => ({
	...(await original<typeof import('$lib/api/client')>()),
	apiJson: vi.fn()
}));
vi.mock('$lib/auth/auth', () => ({
	authErrorHint: () => 'Не получилось скачать архив. Попробуйте ещё раз.'
}));

const pageStore = testPage.store as Writable<{ url: URL }>;
let nextId = 0;
let dispose: (() => Promise<void>) | undefined;
beforeEach(() => {
	vi.mocked(apiJson).mockReset();
	vi.mocked(openDownload).mockReset();
	vi.mocked(goto).mockReset().mockResolvedValue(undefined);
});
afterEach(async () => {
	await dispose?.();
	dispose = undefined;
	vi.useRealTimers();
});

function fixture(active = true, origin = '') {
	const id = `archive-test-${++nextId}`;
	const circle = {
		origin,
		circleId: id,
		name: 'Семья',
		color: 'terracotta',
		archiveCycle: active
			? {
					active: true,
					cutoff_date: '2026-10-01',
					deadline: '2026-11-20T12:00:00Z',
					cutoff_locked: true,
					personal_archive_bytes: 1024,
					personal_archive_media_count: 1,
					personal_archive_post_count: 2,
					download_url: `/api/v1/circles/${id}/archive/download`
				}
			: null
	} as unknown as CircleContext;
	return { id, circle, url: `/api/v1/circles/${id}/archive/download` };
}

function showArchive(circle: CircleContext) {
	history.replaceState({}, '', `/circles/${circle.circleId}/archive`);
	const target = document.createElement('div');
	document.body.append(target);
	const component = mount(ArchivePage, { target, context: new Map([[CIRCLE_CTX, circle]]) });
	flushSync();
	dispose = async () => {
		await unmount(component);
		target.remove();
	};
	return target;
}

function downloadButton(target: ParentNode): HTMLButtonElement {
	const found = [...target.querySelectorAll('button')].find((b) =>
		b.textContent?.trim().startsWith('Скачать')
	);
	expect(found).toBeDefined();
	return found!;
}

// A repeated request stays in Wynd: no new browser download, a real Retry-After
// countdown and an enabled retry after it ends, with the same archive data.
it('shows approved 6.23a and allows a new link after the server retry interval', async () => {
	vi.useFakeTimers();
	const { circle, url } = fixture();
	const target = showArchive(circle);
	vi.mocked(apiJson).mockRejectedValueOnce(new ApiError(429, 'rate_limited', undefined, 30));
	downloadButton(target).click();
	await vi.waitFor(() => expect(target.textContent).toContain('Скачивание уже началось.'));
	expect(downloadButton(target).disabled).toBe(true);
	expect(target.textContent).toContain('через 30 секунд');
	expect(openDownload).not.toHaveBeenCalled();
	expect(goto).not.toHaveBeenCalled();
	await vi.advanceTimersByTimeAsync(30_000);
	flushSync();
	expect(downloadButton(target).disabled).toBe(false);
	expect(target.textContent).not.toContain('через 30 секунд');
	vi.mocked(apiJson).mockResolvedValueOnce({ url: '/api/v1/archive/fresh-token' });
	downloadButton(target).click();
	await vi.waitFor(() => expect(openDownload).toHaveBeenCalledWith('/api/v1/archive/fresh-token'));
	expect(apiJson).toHaveBeenLastCalledWith(
		'',
		url.replace('/api/v1', '').replace('/download', '/link'),
		{ method: 'POST' }
	);
	expect(target.textContent).not.toContain('Скачивание уже началось.');
	expect(target.textContent).toContain('2 записи');
});

it('takes a busy banner request to the archive page, retaining the server and retry period', async () => {
	const { id, circle, url } = fixture(true, 'https://other.example');
	history.replaceState({}, '', `/circles/${id}`);
	vi.mocked(apiJson).mockRejectedValueOnce(new ApiError(429, 'rate_limited', undefined, 45));
	await startArchiveDownload(circle.origin, url);
	expect(goto).toHaveBeenCalledWith(`/circles/${id}/archive`);
	expect(openDownload).not.toHaveBeenCalled();
	await startArchiveDownload(circle.origin, url);
	expect(apiJson).toHaveBeenCalledTimes(1);
	const target = showArchive(circle);
	expect(target.textContent).toContain('Скачивание уже началось.');
	expect(target.textContent).toContain('45 секунд');
	expect(downloadButton(target).disabled).toBe(true);
});

it('uses the same busy state outside an archiving cycle without inventing dates', async () => {
	const { circle, url } = fixture(false);
	const target = showArchive(circle);
	vi.mocked(apiJson).mockRejectedValueOnce(new ApiError(429, 'rate_limited', undefined, 30));
	await startArchiveDownload(circle.origin, url);
	flushSync();
	expect(target.textContent).toContain('Скачивание уже началось.');
	expect(target.textContent).not.toContain('Снято на');
	expect(target.textContent).not.toContain('Скачать до');
});

it.each([true, false])(
	'keeps the button stable while requesting a link (active cycle: %s)',
	async (active) => {
		const { circle } = fixture(active);
		const target = showArchive(circle);
		const button = downloadButton(target);
		const label = button.textContent;
		const classes = button.className;
		let resolve!: (value: { url: string }) => void;
		vi.mocked(apiJson).mockReturnValueOnce(
			new Promise((r) => {
				resolve = r;
			})
		);
		button.click();
		flushSync();
		expect(button.textContent).toBe(label);
		expect(button.className).toBe(classes);
		expect(button.disabled).toBe(false);
		button.click();
		expect(apiJson).toHaveBeenCalledTimes(1);
		resolve({ url: '/api/v1/archive/only-one' });
		await vi.waitFor(() => expect(openDownload).toHaveBeenCalledTimes(1));
		expect(button.textContent).toBe(label);
		expect(button.className).toBe(classes);
	}
);

it('keeps the state of identical circle IDs on different servers independent', async () => {
	const { id, url } = fixture();
	vi.mocked(apiJson).mockRejectedValueOnce(new ApiError(429, 'rate_limited', undefined, 30));
	await startArchiveDownload('https://one.example', url);
	vi.mocked(apiJson).mockResolvedValueOnce({ url: '/api/v1/archive/two' });
	await startArchiveDownload('https://two.example', url);
	expect(openDownload).toHaveBeenCalledWith('https://two.example/api/v1/archive/two');
	expect(archiveDownloadState('https://one.example', id)?.retryAt).toBeGreaterThan(Date.now());
	expect(archiveDownloadState('https://two.example', id)).toBeUndefined();
});

it('shows a failed link request inside Wynd instead of an unhandled promise or browser page', async () => {
	const { circle, url } = fixture();
	const target = showArchive(circle);
	vi.mocked(apiJson).mockRejectedValueOnce(new TypeError('network failure'));
	await startArchiveDownload(circle.origin, url);
	flushSync();
	expect(target.textContent).toContain('Не получилось скачать архив. Попробуйте ещё раз.');
	expect(downloadButton(target).disabled).toBe(false);
	expect(openDownload).not.toHaveBeenCalled();
});

// 6.23b is a public explanation after token validation, with no account API,
// login gate, back action or fresh download that could disturb the first one.
it('renders 6.23b without a session and does not restart its countdown every tick', async () => {
	vi.useFakeTimers();
	pageStore.set({ url: new URL('http://localhost/archive/busy?retry_after=3') });
	const target = document.createElement('div');
	document.body.append(target);
	const component = mount(BusyPage, { target });
	dispose = async () => {
		await unmount(component);
		target.remove();
	};
	flushSync();
	expect(target.textContent).toContain('Архив уже скачивается');
	expect(target.textContent).toContain('в Wynd через 3 секунды');
	expect(target.querySelector('button[aria-label="Назад"]')).toBeNull();
	expect(target.textContent).not.toContain('Войти');
	expect(apiJson).not.toHaveBeenCalled();
	await vi.advanceTimersByTimeAsync(3_000);
	flushSync();
	expect(target.textContent).not.toContain('через');
	expect(target.textContent).toContain('Эту страницу можно закрыть.');
	expect(openDownload).not.toHaveBeenCalled();
});

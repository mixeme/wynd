import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount } from 'svelte';
import MovePage from '../../routes/settings/servers/accept/move/+page.svelte';
import RevivePage from '../../routes/settings/servers/accept/+page.svelte';
import { apiJson, ApiError } from '$lib/api/client';
import { replaceState } from '$app/navigation';
import { loadArchive, openFile, archiveOperationKey, type LoadedArchive } from './revive';

vi.mock('$app/navigation', () => ({
	goto: vi.fn(),
	replaceState: vi.fn(),
	afterNavigate: vi.fn(),
	beforeNavigate: vi.fn()
}));
vi.mock('$app/stores', async () => {
	const { readable } = await import('svelte/store');
	return { page: readable({ url: new URL('http://localhost/settings/servers/accept') }) };
});
vi.mock('$lib/api/client', async (original) => ({
	...(await original<typeof import('$lib/api/client')>()),
	apiJson: vi.fn()
}));
vi.mock('$lib/archive/revive', async (original) => ({
	...(await original<typeof import('$lib/archive/revive')>()),
	openFile: vi.fn(),
	loadArchive: vi.fn()
}));
vi.mock('$lib/instance/serverCard', () => ({
	loadServerCard: vi.fn(async (origin: string) => ({
		info: { name: origin ? 'Новый сервер' : 'Прежний сервер', allow_name_erasure: true }
	}))
}));
vi.mock('$lib/instance/source.svelte', () => ({
	loadSourceUrl: vi.fn(),
	sourceUrl: () => 'https://example.org/source'
}));
vi.mock('$lib/session/session.svelte', async (original) => ({
	...(await original<typeof import('$lib/session/session.svelte')>()),
	loadSessions: vi.fn(async () => [
		{ origin: '', name: 'Прежний сервер', email: 'old@example.org' },
		{ origin: 'https://next.example', name: 'Новый сервер', email: 'new@example.org' }
	])
}));

const link = 'https://source.example/move/ABCD-EFGH-IJKL-MNOP';
const archive = {
	manifest: {
		circle_name: 'Семья',
		server: 'https://source.example',
		snapshot_at: '2026-11-02T12:00:00Z'
	},
	tables: { circles: { columns: ['id'], rows: [['source-circle']] } },
	posts: 3,
	members: 2,
	circleBytes: 5000,
	circleColor: 'terracotta',
	emails: ['old@example.org', 'new@example.org'],
	people: ['Кот', 'Аня']
} as unknown as LoadedArchive;
let requestKey: string;
let withdrawn: boolean;
let failWithdrawal: boolean;
let dispose: (() => Promise<void>) | undefined;

beforeEach(() => {
	vi.mocked(apiJson).mockReset();
	vi.mocked(replaceState).mockClear();
	vi.mocked(loadArchive).mockClear();
	vi.mocked(openFile).mockClear();
	withdrawn = false;
	failWithdrawal = false;
	requestKey = 'move:source-circle';
	vi.mocked(openFile).mockResolvedValue({ encrypted: false, src: {} } as never);
	vi.mocked(loadArchive).mockResolvedValue(archive);
	vi.mocked(apiJson).mockImplementation(async (origin, path, init) => {
		const room = origin
			? { room_bytes: 8000, fits: true, limit: 'quota' }
			: {
					room_bytes: 1000,
					fits: false,
					limit: 'quota',
					...(!withdrawn
						? {
								request: {
									id: 'request-old',
									operation_key: requestKey,
									status: 'pending',
									circle_name: 'Семья',
									requested_bytes: 5000,
									created_at: '2026-11-02T12:00:00Z'
								}
							}
						: {})
				};
		const move = {
			send_invites: false,
			id: 'draft',
			operation_key: 'move:source-circle',
			status: 'draft',
			link,
			source_url: 'https://source.example',
			source_name: 'Источник',
			expires_at: '2026-11-05T12:00:00Z',
			link_alive: true,
			circle: { name: 'Семья', color: 'terracotta', posts: 3, members: 2, bytes: 5000 },
			people: [],
			...room
		};
		if (path === '/moves/incoming') return init?.method === 'POST' ? move : { move };
		if (path.startsWith('/revive/room')) return room;
		if (path === '/revive/invitable') return { invitable: origin ? 0 : 1 };
		if (path === '/revive/place_requests/request-old') {
			if (failWithdrawal) throw new Error('offline');
			withdrawn = true;
			return { status: 'withdrawn' };
		}
		throw new Error(`Unexpected request ${origin} ${path}`);
	});
});
afterEach(async () => {
	await dispose?.();
	dispose = undefined;
});

async function open(kind: 'move' | 'archive') {
	const target = document.createElement('div');
	document.body.append(target);
	const instance = mount(kind === 'move' ? MovePage : RevivePage, { target });
	dispose = async () => {
		await unmount(instance);
		target.remove();
	};
	await vi.waitFor(() => expect(target.querySelector('button.server-card')).not.toBeNull());
	if (kind === 'archive') {
		requestKey = archiveOperationKey(archive);
		const field = target.querySelector('input[type=file]')!;
		Object.defineProperty(field, 'files', { value: [new File(['archive'], 'family.wynd')] });
		field.dispatchEvent(new Event('change', { bubbles: true }));
		await vi.waitFor(() => expect(target.textContent).toContain('Место попросили'));
	}
	return target;
}
function button(target: HTMLElement, text: string) {
	return [...target.querySelectorAll('button')].find((el) => el.textContent?.includes(text))!;
}
async function choose(target: HTMLElement) {
	(target.querySelector('button.server-card') as HTMLButtonElement).click();
	await vi.waitFor(() => expect(target.textContent).toContain('Ваши серверы'));
	await vi.waitFor(() => expect(button(target, 'Новый сервер')).toBeDefined());
	await vi.waitFor(() => expect(target.textContent).not.toContain('Проверяем связь…'));
	button(target, 'Новый сервер').click();
	flushSync();
}

describe('receiving server', () => {
	it('повторное нажатие не отправляет второй отзыв', async () => {
		const run = vi.mocked(apiJson).getMockImplementation()!;
		let finish!: () => void;
		const pending = new Promise<void>((resolve) => {
			finish = resolve;
		});
		vi.mocked(apiJson).mockImplementation(async (...args) => {
			if (args[2]?.method === 'DELETE') await pending;
			return run(...args);
		});
		const target = await open('move');
		await choose(target);
		button(target, 'Новый сервер').click();
		expect(
			vi.mocked(apiJson).mock.calls.filter((call) => call[2]?.method === 'DELETE')
		).toHaveLength(1);
		finish();
		await vi.waitFor(() => expect(replaceState).toHaveBeenCalled());
	});
	it('сохраняет ссылку и выбранный сервер при ошибке проверки ссылки', async () => {
		const run = vi.mocked(apiJson).getMockImplementation()!;
		vi.mocked(apiJson).mockImplementation(async (...args) => {
			if (args[0] && args[1] === '/moves/incoming') throw new ApiError(502, 'source_unreachable');
			return run(...args);
		});
		const target = await open('move');
		await choose(target);
		await vi.waitFor(() => expect(replaceState).toHaveBeenCalled());
		expect((target.querySelector('input') as HTMLInputElement).value).toBe(link);
		expect(target.textContent).toContain('Сервер по ссылке не отвечает');
	});
	for (const kind of ['move', 'archive'] as const) {
		it(`${kind}: сохраняет форму и отзывает прежнюю просьбу до смены`, async () => {
			const target = await open(kind);
			await choose(target);
			await vi.waitFor(() => expect(replaceState).toHaveBeenCalled());
			expect(withdrawn).toBe(true);
			expect(target.textContent).toContain('Новый сервер');
			if (kind === 'move')
				expect((target.querySelector('input') as HTMLInputElement).value).toBe(link);
			else {
				expect(loadArchive).toHaveBeenCalledTimes(1);
				expect(target.textContent).toContain('Семья');
				expect(target.querySelector<HTMLButtonElement>('[role=switch]')?.disabled).toBe(true);
			}
			const writes = vi
				.mocked(apiJson)
				.mock.calls.filter((call) => ['POST', 'DELETE'].includes(call[2]?.method ?? ''));
			const withdrawalIndex = writes.findIndex((call) => call[2]?.method === 'DELETE');
			expect(writes[withdrawalIndex].slice(0, 2)).toEqual([
				'',
				'/revive/place_requests/request-old'
			]);
			expect(
				writes.slice(0, withdrawalIndex).some((call) => call[0] === 'https://next.example')
			).toBe(false);
			expect(writes.some((call) => call[1] === '/revive' || call[1].endsWith('/start'))).toBe(
				false
			);
		});
	}
	it('сбой отзыва сохраняет выбор и ссылку, повтор успешен', async () => {
		failWithdrawal = true;
		const target = await open('move');
		await choose(target);
		await vi.waitFor(() =>
			expect(target.textContent).toContain('Не удалось отозвать прежнюю просьбу')
		);
		expect(replaceState).not.toHaveBeenCalled();
		expect(
			vi.mocked(apiJson).mock.calls.some((call) => call[0] && call[2]?.method === 'POST')
		).toBe(false);
		failWithdrawal = false;
		button(target, 'Новый сервер').click();
		await vi.waitFor(() => expect(replaceState).toHaveBeenCalled());
		expect((target.querySelector('input') as HTMLInputElement).value).toBe(link);
	});
	it('не отзывает просьбу другого одноимённого круга', async () => {
		requestKey = 'move:another-circle';
		const target = await open('move');
		await choose(target);
		await vi.waitFor(() => expect(replaceState).toHaveBeenCalled());
		expect(withdrawn).toBe(false);
	});
	it('выбор текущего сервера не отзывает просьбу', async () => {
		const target = await open('move');
		(target.querySelector('button.server-card') as HTMLButtonElement).click();
		await vi.waitFor(() => expect(target.textContent).toContain('Ваши серверы'));
		button(target, 'Прежний сервер').click();
		await vi.waitFor(() => expect(target.textContent).not.toContain('Ваши серверы'));
		expect(withdrawn).toBe(false);
	});
});

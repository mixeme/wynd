import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { flushSync, mount, unmount, type Component } from 'svelte';
import NewPage from '../../routes/circles/new/+page.svelte';
import ServerPage from '../../routes/circles/new/server/+page.svelte';
import YouPage from '../../routes/circles/new/you/+page.svelte';
import { NEW_CIRCLE_CTX, type NewCircleContext } from './new-circle';
import { createCircle } from './circles';

const browser = vi.hoisted(() => ({ entries: [] as string[] }));
vi.mock('$app/navigation', () => ({
	goto: vi.fn(async (path: string, options?: { replaceState?: boolean }) => {
		if (options?.replaceState) browser.entries[browser.entries.length - 1] = path;
		else browser.entries.push(path);
	}),
	beforeNavigate: vi.fn(),
	afterNavigate: vi.fn()
}));
vi.mock('$app/stores', async () => {
	const { readable } = await import('svelte/store');
	return { page: readable({ url: new URL('http://localhost/circles/new') }) };
});
vi.mock('$lib/navigation/up', () => ({ goUp: vi.fn(), setBackHandler: () => () => {} }));
vi.mock('$lib/instance/source.svelte', () => ({
	loadSourceUrl: vi.fn(),
	sourceUrl: () => 'https://example.com/source'
}));
vi.mock('$lib/auth/auth', () => ({ authErrorHint: () => 'Не получилось создать круг' }));
vi.mock('$lib/circles/circles', () => ({ createCircle: vi.fn() }));
vi.mock('$lib/circles/meta', () => ({
	setCircleColor: vi.fn().mockResolvedValue(undefined),
	setCircleIdentity: vi.fn().mockResolvedValue(undefined)
}));
vi.mock('$lib/circles/origin', () => ({ rememberCircleOrigin: vi.fn() }));
vi.mock('$lib/circles/settings', () => ({ setIdentityAvatar: vi.fn() }));
vi.mock('$lib/journal/posts', () => ({ createPost: vi.fn() }));

let close: (() => Promise<void>) | undefined;
beforeEach(() => {
	browser.entries = ['/circles', '/circles/new'];
	vi.mocked(createCircle)
		.mockReset()
		.mockResolvedValue({ id: 'created' } as never);
});
afterEach(async () => {
	await close?.();
	close = undefined;
});

function fixture(diaryMode = false): NewCircleContext {
	const form = $state<NewCircleContext>({
		name: 'Семья',
		color: 'ochre',
		editWindow: '1h',
		customHours: 24,
		diaryMode,
		selectedOrigin: '',
		sessions: [
			{ origin: '', name: 'Свой сервер', email: 'a@example.com', token: 'test', account_id: 'a' },
			{
				origin: 'https://other.example',
				name: 'Другой сервер',
				email: 'a@example.com',
				token: 'test',
				account_id: 'b'
			}
		],
		serverCards: {},
		circleCounts: {},
		serverSubtitle: (s) => s.name,
		serverDetail: () => undefined,
		loading: false,
		error: '',
		ready: true
	});
	return form;
}

async function show(page: Component, form: NewCircleContext) {
	await close?.();
	const target = document.createElement('div');
	document.body.append(target);
	const component = mount(page, { target, context: new Map([[NEW_CIRCLE_CTX, form]]) });
	flushSync();
	close = async () => {
		await unmount(component);
		target.remove();
	};
	return target;
}

function click(target: ParentNode, text: string) {
	const button = [...target.querySelectorAll('button')].find((b) => b.textContent?.trim() === text);
	expect(button, text).toBeDefined();
	button!.click();
	flushSync();
}

it.each([false, true])(
	'leaves no completed creation step behind the circle (diary: %s)',
	async (diary) => {
		const form = fixture(diary);
		const first = await show(NewPage, form);
		click(first, diary ? 'Завести дневник' : 'Создать и позвать');
		const identity = await show(YouPage, form);
		const input = identity.querySelector<HTMLInputElement>('input[type="text"]')!;
		input.value = 'Аня';
		input.dispatchEvent(new Event('input', { bubbles: true }));
		flushSync();
		click(identity, diary ? 'Завести дневник' : 'Создать и позвать');
		const destination = diary ? '/circles/created' : '/circles/created/settings/invite?from=create';
		await vi.waitFor(() => expect(browser.entries.at(-1)).toBe(destination));
		expect(browser.entries).toEqual(['/circles', destination]);
	}
);

it('keeps one creation entry even after choosing a server repeatedly', async () => {
	const form = fixture();
	for (let i = 0; i < 2; i++) {
		const first = await show(NewPage, form);
		first.querySelector<HTMLButtonElement>('button.server-card')!.click();
		flushSync();
		const picker = await show(ServerPage, form);
		const rows = picker.querySelectorAll<HTMLButtonElement>('button.row2');
		rows[i % 2].click();
		flushSync();
	}
	expect(browser.entries).toEqual(['/circles', '/circles/new?origin=https%3A%2F%2Fother.example']);
	expect(form.name).toBe('Семья');
});

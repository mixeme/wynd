// Черновик адреса стартовой страницы: возвращается после смерти процесса в фоне
// и стирается при подтверждённом выборе (дефект владельца 20.11, план 50).
import { mount, tick, unmount } from 'svelte';
import { afterAll, afterEach, beforeAll, describe, expect, it } from 'vitest';

const DRAFT_KEY = 'wynd-startup-draft';

type Request = { op: string; address?: string; id: number };
type Bridge = {
	postMessage(message: string): void;
	onmessage: ((event: { data: string }) => void) | null;
};

let outgoing: Request[] = [];
// Мост ставится до импорта страницы: она читает native один раз при загрузке модуля.
const bridge: Bridge = {
	onmessage: null,
	postMessage(message: string) {
		outgoing.push(JSON.parse(message) as Request);
	}
};
(window as unknown as { __wyndStartup?: Bridge }).__wyndStartup = bridge;

let Page: typeof import('../../routes/dev/smoke/android-startup/+page.svelte').default;
let root: HTMLElement;
let instance: ReturnType<typeof mount> | null = null;

beforeAll(async () => {
	({ default: Page } = await import('../../routes/dev/smoke/android-startup/+page.svelte'));
});
afterAll(() => {
	delete (window as unknown as { __wyndStartup?: Bridge }).__wyndStartup;
});

function startPage(): { input: () => HTMLInputElement } {
	root = document.createElement('div');
	document.body.append(root);
	instance = mount(Page, { target: root });
	return {
		input: () => {
			const input = root.querySelector<HTMLInputElement>('input[type="text"]');
			if (!input) throw new Error('адресное поле не найдено');
			return input;
		}
	};
}

function replyLatest(op: string, payload: Record<string, unknown> = {}) {
	const id = outgoing.filter((r) => r.op === op).at(-1)?.id;
	if (!id) throw new Error(`нет запроса ${op}`);
	bridge.onmessage?.({ data: JSON.stringify({ id, protocol: 1, ...payload }) });
}

function typeInto(input: HTMLInputElement, value: string) {
	const set = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
	if (!set) throw new Error('value setter недоступен');
	set.call(input, value);
	input.dispatchEvent(new Event('input', { bubbles: true }));
}

function pressNext(): HTMLButtonElement {
	const next = [...root.querySelectorAll('button')].find((b) => b.textContent?.trim() === 'Дальше');
	if (!(next instanceof HTMLButtonElement)) throw new Error('кнопка «Дальше» не найдена');
	next.click();
	return next;
}

afterEach(() => {
	if (instance) {
		unmount(instance);
		instance = null;
	}
	localStorage.removeItem(DRAFT_KEY);
	document.body.replaceChildren();
	outgoing = [];
});

describe('черновик адреса стартовой страницы', () => {
	it('возвращается в поле после смерти процесса в фоне', async () => {
		localStorage.setItem(DRAFT_KEY, 'home.example.org');
		const { input } = startPage();
		await tick();
		replyLatest('state', { offline: false, selected: null });
		await tick();
		await tick();
		expect(input().value).toBe('home.example.org');
	});

	it('набранный адрес сохраняется по мере ввода', async () => {
		const { input } = startPage();
		await tick();
		replyLatest('state', { offline: false, selected: null });
		await tick();
		await tick();
		typeInto(input(), 'other.example.org');
		await tick();
		expect(localStorage.getItem(DRAFT_KEY)).toBe('other.example.org');
	});

	it('стирается после подтверждённого выбора сервера', async () => {
		localStorage.setItem(DRAFT_KEY, 'other.example.org');
		const { input } = startPage();
		await tick();
		replyLatest('state', { offline: false, selected: null });
		await tick();
		await tick();
		typeInto(input(), 'other.example.org');
		pressNext();
		await tick();
		replyLatest('check', { preview: { name: 'Дом Ани', origin: 'https://other.example.org' } });
		await tick();
		await tick();
		pressNext();
		await tick();
		replyLatest('open', {});
		await tick();
		await tick();
		expect(localStorage.getItem(DRAFT_KEY)).toBeNull();
	});
});

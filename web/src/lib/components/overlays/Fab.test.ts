import { createRawSnippet, flushSync, mount, unmount } from 'svelte';
import { describe, expect, it, vi } from 'vitest';
import Fab from './Fab.svelte';

const plus = createRawSnippet(() => ({ render: () => '<span>+</span>', setup: () => () => {} }));

function key(k: string) {
	window.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true }));
}

// Инвариант (план 42, UI-3): открытое меню плюса — role="menu", фокус на
// первом пункте, стрелки ходят по кругу, Escape зовёт onclose.
describe('Fab menu', () => {
	it('focuses items, cycles with arrows and closes on Escape', () => {
		const onclose = vi.fn();
		const target = document.createElement('div');
		document.body.append(target);
		const items = [
			{ label: 'Новый круг', onclick: vi.fn() },
			{ label: 'Новая группа', onclick: vi.fn() }
		];
		const instance = mount(Fab, { target, props: { children: plus, menuOpen: true, items, onclose } });
		flushSync();
		const menu = target.querySelector('[role="menu"]');
		const buttons = [...target.querySelectorAll<HTMLButtonElement>('[role="menuitem"]')];
		expect(menu).not.toBeNull();
		expect(document.activeElement).toBe(buttons[0]);
		key('ArrowDown');
		expect(document.activeElement).toBe(buttons[1]);
		key('ArrowDown');
		expect(document.activeElement).toBe(buttons[0]);
		key('ArrowUp');
		expect(document.activeElement).toBe(buttons[1]);
		key('Escape');
		expect(onclose).toHaveBeenCalledOnce();
		unmount(instance);
		target.remove();
	});
});

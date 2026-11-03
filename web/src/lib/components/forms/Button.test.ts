import { createRawSnippet, mount, unmount } from 'svelte';
import { describe, expect, it, vi } from 'vitest';
import Button from './Button.svelte';

const children = createRawSnippet(() => ({
	render: () => '<span>Click</span>',
	setup: () => () => {}
}));

describe('Button', () => {
	it('calls onclick on click', () => {
		const onclick = vi.fn();
		const target = document.createElement('div');
		const instance = mount(Button, { target, props: { onclick, children } });
		const btn = target.querySelector('button');
		expect(btn?.tagName).toBe('BUTTON');
		btn?.click();
		expect(onclick).toHaveBeenCalledOnce();
		unmount(instance);
	});

	it('disabled blocks click', () => {
		const onclick = vi.fn();
		const target = document.createElement('div');
		const instance = mount(Button, {
			target,
			props: { onclick, disabled: true, children }
		});
		const btn = target.querySelector('button');
		expect(btn?.disabled).toBe(true);
		expect(btn?.classList.contains('off')).toBe(true);
		btn?.click();
		expect(onclick).not.toHaveBeenCalled();
		unmount(instance);
	});

	it('loading blocks click', () => {
		const onclick = vi.fn();
		const target = document.createElement('div');
		const instance = mount(Button, {
			target,
			props: { onclick, loading: true, children }
		});
		const btn = target.querySelector('button');
		expect(btn?.disabled).toBe(true);
		expect(btn?.classList.contains('off')).toBe(true);
		expect(btn?.textContent).toBe('…');
		btn?.click();
		expect(onclick).not.toHaveBeenCalled();
		unmount(instance);
	});

	it('keeps the busy label inside the disabled button', () => {
		const onclick = vi.fn();
		const target = document.createElement('div');
		const instance = mount(Button, {
			target,
			props: { onclick, loading: true, loadingLabel: 'Подключаемся…', children }
		});
		const btn = target.querySelector('button');
		expect(btn?.textContent).toBe('Подключаемся…');
		expect(target.textContent).toBe(btn?.textContent);
		expect(btn?.getAttribute('aria-busy')).toBe('true');
		expect(btn?.disabled).toBe(true);
		btn?.click();
		expect(onclick).not.toHaveBeenCalled();
		unmount(instance);
	});

	it('shows the action label while idle even with a busy label', () => {
		const target = document.createElement('div');
		const instance = mount(Button, {
			target,
			props: { onclick: vi.fn(), loadingLabel: 'Подключаемся…', children }
		});
		const btn = target.querySelector('button');
		expect(btn?.textContent).toBe('Click');
		expect(btn?.disabled).toBe(false);
		expect(btn?.hasAttribute('aria-busy')).toBe(false);
		unmount(instance);
	});
});

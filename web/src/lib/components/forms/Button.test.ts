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
});

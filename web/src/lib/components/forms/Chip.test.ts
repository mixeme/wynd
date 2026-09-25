import { createRawSnippet, mount, unmount } from 'svelte';
import { describe, expect, it, vi } from 'vitest';
import Chip from './Chip.svelte';

const children = createRawSnippet(() => ({
	render: () => '<span>Chip</span>',
	setup: () => () => {}
}));

describe('Chip', () => {
	it('renders span without onclick', () => {
		const target = document.createElement('div');
		const instance = mount(Chip, { target, props: { children } });
		expect(target.querySelector('span')?.tagName).toBe('SPAN');
		expect(target.querySelector('button')).toBeNull();
		unmount(instance);
	});

	it('renders button with onclick', () => {
		const onclick = vi.fn();
		const target = document.createElement('div');
		const instance = mount(Chip, { target, props: { children, onclick } });
		const btn = target.querySelector('button');
		expect(btn?.tagName).toBe('BUTTON');
		btn?.click();
		expect(onclick).toHaveBeenCalledOnce();
		unmount(instance);
	});
});

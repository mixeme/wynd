import { mount, unmount } from 'svelte';
import { describe, expect, it, vi } from 'vitest';
import MemberRow from './MemberRow.svelte';

describe('MemberRow', () => {
	it('renders div without onclick', () => {
		const target = document.createElement('div');
		const instance = mount(MemberRow, {
			target,
			props: { initial: 'А', name: 'Аня' }
		});
		expect(target.querySelector('div.row2')?.tagName).toBe('DIV');
		expect(target.querySelector('button')).toBeNull();
		unmount(instance);
	});

	it('renders button with onclick', () => {
		const onclick = vi.fn();
		const target = document.createElement('div');
		const instance = mount(MemberRow, {
			target,
			props: { initial: 'А', name: 'Аня', onclick }
		});
		const btn = target.querySelector('button');
		expect(btn?.tagName).toBe('BUTTON');
		btn?.click();
		expect(onclick).toHaveBeenCalledOnce();
		unmount(instance);
	});

	it('hides menu when the row is clickable', () => {
		const onclick = vi.fn();
		const onmenu = vi.fn();
		const target = document.createElement('div');
		const instance = mount(MemberRow, {
			target,
			props: { initial: 'А', name: 'Аня', onclick, menu: true, onmenu }
		});
		expect(target.querySelectorAll('button')).toHaveLength(1);
		unmount(instance);
	});
});

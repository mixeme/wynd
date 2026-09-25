import { mount, unmount } from 'svelte';
import { describe, expect, it, vi } from 'vitest';
import SettingsRow from './SettingsRow.svelte';

describe('SettingsRow', () => {
	it('renders div without onclick', () => {
		const target = document.createElement('div');
		const instance = mount(SettingsRow, { target, props: { title: 'Test' } });
		expect(target.querySelector('div.row2')?.tagName).toBe('DIV');
		expect(target.querySelector('button')).toBeNull();
		unmount(instance);
	});

	it('renders button with onclick', () => {
		const onclick = vi.fn();
		const target = document.createElement('div');
		const instance = mount(SettingsRow, {
			target,
			props: { title: 'Test', onclick }
		});
		const btn = target.querySelector('button');
		expect(btn?.tagName).toBe('BUTTON');
		btn?.click();
		expect(onclick).toHaveBeenCalledOnce();
		unmount(instance);
	});
});

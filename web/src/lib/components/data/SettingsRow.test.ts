import { createRawSnippet, mount, unmount } from 'svelte';
import { describe, expect, it, vi } from 'vitest';
import SettingsRow from './SettingsRow.svelte';

const control = createRawSnippet(() => ({
	render: () => '<button type="button" role="switch" class="sw"></button>',
	setup: () => () => {}
}));

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

	it('renders div.row2 with control and button.sw inside', () => {
		const target = document.createElement('div');
		const instance = mount(SettingsRow, {
			target,
			props: { title: 'Новые записи', control }
		});
		expect(target.querySelector('div.row2')?.tagName).toBe('DIV');
		expect(target.querySelector('button.sw')?.getAttribute('role')).toBe('switch');
		expect(target.querySelector('button.row2')).toBeNull();
		unmount(instance);
	});

	it('ignores onclick when control is set', () => {
		const onclick = vi.fn();
		const target = document.createElement('div');
		const instance = mount(SettingsRow, {
			target,
			props: { title: 'Test', onclick, control }
		});
		expect(target.querySelector('div.row2')?.tagName).toBe('DIV');
		expect(target.querySelector('button.row2')).toBeNull();
		unmount(instance);
	});
});

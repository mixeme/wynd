import { mount, unmount } from 'svelte';
import { describe, expect, it } from 'vitest';
import FieldDisplay from './FieldDisplay.svelte';

describe('FieldDisplay', () => {
	it('uses fld by default', () => {
		const target = document.createElement('div');
		const instance = mount(FieldDisplay, { target, props: { value: 'home' } });
		const el = target.querySelector('div');
		expect(el?.classList.contains('fld')).toBe(true);
		expect(el?.classList.contains('inp')).toBe(false);
		expect(el?.textContent).toBe('home');
		unmount(instance);
	});

	it('uses inp when admin', () => {
		const target = document.createElement('div');
		const instance = mount(FieldDisplay, {
			target,
			props: { admin: true, mono: true, value: 'https://example/join/x' }
		});
		const el = target.querySelector('div');
		expect(el?.classList.contains('inp')).toBe(true);
		expect(el?.classList.contains('fld')).toBe(false);
		expect(el?.classList.contains('mono')).toBe(true);
		unmount(instance);
	});
});

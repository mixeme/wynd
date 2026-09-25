import { mount, unmount } from 'svelte';
import { describe, expect, it } from 'vitest';
import AdminNav from './AdminNav.svelte';

describe('AdminNav', () => {
	it('renders spans without links', () => {
		const target = document.createElement('div');
		const instance = mount(AdminNav, { target, props: { active: 'Доступ' } });
		expect(target.querySelector('button')).toBeNull();
		expect(target.querySelectorAll('span.admnav > span')).toHaveLength(6);
		unmount(instance);
	});

	it('renders buttons with links and no role=button', () => {
		const target = document.createElement('div');
		const instance = mount(AdminNav, { target, props: { active: 'Доступ', links: true } });
		const buttons = target.querySelectorAll('button');
		expect(buttons).toHaveLength(6);
		expect(target.querySelector('[role="button"]')).toBeNull();
		expect(buttons[3]?.classList.contains('on')).toBe(true);
		unmount(instance);
	});
});

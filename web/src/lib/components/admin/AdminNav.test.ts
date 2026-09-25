import { mount, unmount } from 'svelte';
import { describe, expect, it } from 'vitest';
import AdminNav, { ADMIN_NAV } from './AdminNav.svelte';

describe('AdminNav', () => {
	it('lists sections in panel order', () => {
		expect([...ADMIN_NAV]).toEqual([
			'Проверка',
			'Общие',
			'Доступ',
			'Люди',
			'Хранилище',
			'Сжатие',
			'Оплата'
		]);
	});

	it('renders spans without links', () => {
		const target = document.createElement('div');
		const instance = mount(AdminNav, { target, props: { active: 'Доступ' } });
		expect(target.querySelector('button')).toBeNull();
		const items = [...target.querySelectorAll('span.admnav > span')];
		expect(items.map((el) => el.textContent)).toEqual([...ADMIN_NAV]);
		unmount(instance);
	});

	it('renders buttons with links and no role=button', () => {
		const target = document.createElement('div');
		const instance = mount(AdminNav, { target, props: { active: 'Доступ', links: true } });
		const buttons = target.querySelectorAll('button');
		expect(buttons).toHaveLength(7);
		expect(target.querySelector('[role="button"]')).toBeNull();
		expect(buttons[2]?.classList.contains('on')).toBe(true);
		unmount(instance);
	});
});

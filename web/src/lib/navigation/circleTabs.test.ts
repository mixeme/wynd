import { describe, expect, it } from 'vitest';
import { tabTap } from './circleTabs';

describe('tabTap (C106)', () => {
	it('другая вкладка — переход', () => {
		expect(tabTap('Хронология', 'Дни', true, true)).toBe('navigate');
	});
	it('повтор на самой вкладке — прокрутка наверх', () => {
		expect(tabTap('Дни', 'Дни', true, true)).toBe('scroll');
	});
	it('та же вкладка на чужом экране (день) — переход в список дней', () => {
		expect(tabTap('Дни', 'Дни', false, true)).toBe('navigate');
	});
	it('без круга вкладки не работают', () => {
		expect(tabTap('Дни', 'Сетка', true, false)).toBe('none');
	});
});

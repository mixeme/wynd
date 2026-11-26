import { describe, expect, it } from 'vitest';
import { isCircleSettingsPath } from './wideDetail';

describe('isCircleSettingsPath', () => {
	it('настройки круга и их разделы — слева меню настроек', () => {
		for (const path of [
			'/circles/c1/settings',
			'/circles/c1/settings/notify',
			'/circles/c1/settings/invite/from',
			'/circles/c1/quota',
			'/circles/c1/quota/cleanup',
			'/circles/c1/archive',
			'/circles/c1/split',
			'/circles/c1/move'
		]) {
			expect(isCircleSettingsPath(path, 'c1')).toBe(true);
		}
	});

	it('вкладки и остальные экраны круга — со списком кругов', () => {
		for (const path of [
			'/circles/c1',
			'/circles/c1/days',
			'/circles/c1/grid',
			'/circles/c1/map',
			'/circles/c1/responses',
			'/circles/c1/search',
			'/circles/c1/posts/p1',
			'/circles/c1/compose',
			'/circles/c1/join'
		]) {
			expect(isCircleSettingsPath(path, 'c1')).toBe(false);
		}
	});

	it('чужой круг не путается', () => {
		expect(isCircleSettingsPath('/circles/c2/settings', 'c1')).toBe(false);
	});
});

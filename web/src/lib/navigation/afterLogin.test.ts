import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

import {
	isWantedAdminPath,
	isWantedPath,
	rememberAdminPath,
	rememberWantedPath,
	takeAdminPath,
	takeWantedPath
} from './afterLogin';

describe('после входа — туда, куда шёл', () => {
	beforeEach(() => sessionStorage.clear());

	it('отдаёт запомненный путь один раз', () => {
		rememberWantedPath('/circles/c1/archive', 1000);
		expect(takeWantedPath(2000)).toBe('/circles/c1/archive');
		expect(takeWantedPath(3000)).toBeUndefined();
	});

	it('забывает через полчаса', () => {
		rememberWantedPath('/circles/c1/archive', 0);
		expect(takeWantedPath(31 * 60 * 1000)).toBeUndefined();
	});

	it('только свои экраны', () => {
		expect(isWantedPath('/circles/c1/posts/p1?x=1')).toBe(true);
		for (const bad of [
			'https://evil.example/x',
			'//evil.example',
			'/\\evil',
			'/',
			'/circles',
			'/auth/code',
			'/admin/check'
		]) {
			expect(isWantedPath(bad)).toBe(false);
		}
	});
});

describe('после входа в панель — туда, куда шёл', () => {
	beforeEach(() => sessionStorage.clear());

	it('отдаёт запомненный путь панели один раз, отдельно от входа участника', () => {
		rememberAdminPath('/admin/pay/requests/r1', 1000);
		expect(takeWantedPath(2000)).toBeUndefined();
		expect(takeAdminPath(2000)).toBe('/admin/pay/requests/r1');
		expect(takeAdminPath(3000)).toBeUndefined();
	});

	it('берёт письмо с разделом панели: /admin#requests', () => {
		rememberAdminPath('/admin#requests', 1000);
		expect(takeAdminPath(2000)).toBe('/admin#requests');
	});

	it('забывает через полчаса', () => {
		rememberAdminPath('/admin#requests', 0);
		expect(takeAdminPath(31 * 60 * 1000)).toBeUndefined();
	});

	it('только экраны панели', () => {
		for (const good of ['/admin/pay/requests/r1?x=1', '/admin#requests', '/admin/people/a']) {
			expect(isWantedAdminPath(good)).toBe(true);
		}
		for (const bad of [
			'https://evil.example/x',
			'//evil.example',
			'/\\evil',
			'/',
			'/circles',
			'/admin',
			'/admin/login',
			'/admin/bootstrap',
			'/admin?s=1'
		]) {
			expect(isWantedAdminPath(bad)).toBe(false);
		}
	});
});

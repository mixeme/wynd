import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('$app/navigation', () => ({ goto: vi.fn() }));

import { isWantedPath, rememberWantedPath, takeWantedPath } from './afterLogin';

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
		for (const bad of ['https://evil.example/x', '//evil.example', '/\\evil', '/', '/circles', '/auth/code', '/admin/check']) {
			expect(isWantedPath(bad)).toBe(false);
		}
	});
});

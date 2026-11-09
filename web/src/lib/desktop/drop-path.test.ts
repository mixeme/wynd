import { describe, expect, it } from 'vitest';
import { circleFromPath } from './drop-path';

describe('circle from page address', () => {
	it('находит круг на ленте и внутри круга', () => {
		expect(circleFromPath('/circles/c1')).toBe('c1');
		expect(circleFromPath('/circles/c1/compose')).toBe('c1');
		expect(circleFromPath('/circles/c1/days/2026-11-09')).toBe('c1');
	});

	it('вне круга файл не принимает', () => {
		expect(circleFromPath('/')).toBeNull();
		expect(circleFromPath('/circles')).toBeNull();
		expect(circleFromPath('/settings/app')).toBeNull();
	});

	it('раскодирует адрес круга', () => {
		expect(circleFromPath('/circles/a%20b')).toBe('a b');
	});
});

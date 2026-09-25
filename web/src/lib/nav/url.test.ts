import { describe, expect, it } from 'vitest';
import { numberParam, withParam, withoutParam } from './url';

const base = 'http://localhost/circles/c1/posts/p1/album';

describe('разовые параметры адреса', () => {
	it('число из параметра, иначе запасное', () => {
		expect(numberParam(new URL(`${base}?lb=2`), 'lb')).toBe(2);
		expect(numberParam(new URL(base), 'lb')).toBe(-1);
		expect(numberParam(new URL(`${base}?lb=`), 'lb')).toBe(-1);
		expect(numberParam(new URL(`${base}?lb=abc`), 'lb')).toBe(-1);
		expect(numberParam(new URL(`${base}?lb=abc`), 'lb', 0)).toBe(0);
	});

	it('добавляет параметр к пути', () => {
		expect(withParam('/circles/c1', 'reactions', 'p1')).toBe('/circles/c1?reactions=p1');
		expect(withParam('/album', 'lb', 3)).toBe('/album?lb=3');
	});

	it('убирает параметр и оставляет остальные', () => {
		expect(withoutParam(new URL(`${base}?lb=2&q=да`), 'lb')).toBe(
			'/circles/c1/posts/p1/album?q=%D0%B4%D0%B0'
		);
		expect(withoutParam(new URL(`${base}?lb=2`), 'lb')).toBe('/circles/c1/posts/p1/album');
	});

	it('параметра нет — переходить некуда', () => {
		expect(withoutParam(new URL(base), 'lb')).toBeNull();
	});
});

import { describe, expect, it } from 'vitest';
import { toAccusativeTitle, toDativeName } from './names';

describe('toDativeName', () => {
	it('declines common first names', () => {
		expect(toDativeName('Аня')).toBe('Ане');
		expect(toDativeName('Петя')).toBe('Пете');
		expect(toDativeName('Мама')).toBe('Маме');
		expect(toDativeName('Мышь')).toBe('Мыши');
		expect(toDativeName('Кот')).toBe('Коту');
	});
});

describe('toAccusativeTitle', () => {
	it('declines circle titles from the frame', () => {
		expect(toAccusativeTitle('Семья')).toBe('Семью');
	});
});

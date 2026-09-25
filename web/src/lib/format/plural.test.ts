import { describe, expect, it } from 'vitest';
import { WORD, plural, pluralForm } from './plural';

describe('склонение по числу', () => {
	it('единственное — только на 1, 21, 31…', () => {
		expect(plural(1, WORD.post)).toBe('1 запись');
		expect(plural(21, WORD.post)).toBe('21 запись');
		expect(plural(101, WORD.post)).toBe('101 запись');
	});

	it('на 11 — множественное, а не единственное', () => {
		expect(plural(11, WORD.post)).toBe('11 записей');
		expect(plural(111, WORD.comment)).toBe('111 комментариев');
	});

	it('2–4 — вторая форма', () => {
		expect(plural(2, WORD.day)).toBe('2 дня');
		expect(plural(23, WORD.day)).toBe('23 дня');
		expect(plural(104, WORD.file)).toBe('104 файла');
	});

	it('12–14 — третья, несмотря на хвост', () => {
		expect(plural(12, WORD.day)).toBe('12 дней');
		expect(plural(113, WORD.photo)).toBe('113 фотографий');
	});

	it('ноль и пятёрки — третья', () => {
		expect(plural(0, WORD.comment)).toBe('0 комментариев');
		expect(plural(5, WORD.circle)).toBe('5 кругов');
		expect(plural(100, WORD.person)).toBe('100 человек');
	});

	it('форма отдельно от числа', () => {
		expect(pluralForm(1, WORD.comment)).toBe('комментарий');
		expect(pluralForm(3, WORD.comment)).toBe('комментария');
	});
});

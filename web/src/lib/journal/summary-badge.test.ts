import { describe, expect, it } from 'vitest';
import { daysSummary, gridSummary } from './summary-badge';

describe('сводки «Дней» и «Сетки» — «из N» в родительном (C114)', () => {
	it('формы «дней»: 1, 2, 5, 11, 12, 21, 22', () => {
		expect(daysSummary(1, 1)).toBe('1 день из 1 записи');
		expect(daysSummary(2, 2)).toBe('2 дня из 2 записей');
		expect(daysSummary(5, 5)).toBe('5 дней из 5 записей');
		expect(daysSummary(11, 11)).toBe('11 дней из 11 записей');
		expect(daysSummary(12, 12)).toBe('12 дней из 12 записей');
		expect(daysSummary(21, 21)).toBe('21 день из 21 записи');
		expect(daysSummary(22, 22)).toBe('22 дня из 22 записей');
	});

	it('сводка «Сетки» — и подлежащее, и «из N»', () => {
		expect(gridSummary(2, 2)).toBe('2 записи с фото и видео из 2 записей');
		expect(gridSummary(1, 3)).toBe('1 запись с фото и видео из 3 записей');
		expect(gridSummary(5, 21)).toBe('5 записей с фото и видео из 21 записи');
		expect(gridSummary(101, 100)).toBe('101 запись с фото и видео из 100 записей');
	});
});

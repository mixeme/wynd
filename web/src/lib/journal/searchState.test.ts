import { describe, expect, it } from 'vitest';
import { searchStats, toggleAuthor, togglePeriod, type ChipState } from './searchState';
import type { CircleSearchHit } from './types';

function hit(kind: CircleSearchHit['kind'], id: string): CircleSearchHit {
	return {
		kind,
		post_id: id,
		snippet: 'текст',
		created_at: '2026-08-30T10:00:00Z',
		entry_date: '2026-08-30',
		author_name: 'Аня'
	} as CircleSearchHit;
}

const base: ChipState = { author: '', periodActive: false, periodFrom: '', periodTo: '' };

describe('фильтры поиска', () => {
	it('автор ставится и снимается тем же нажатием', () => {
		const withAuthor = toggleAuthor(base, 'Боб');
		expect(withAuthor.author).toBe('Боб');
		expect(toggleAuthor(withAuthor, 'Боб').author).toBe('');
		expect(toggleAuthor(withAuthor, 'Аня').author).toBe('Аня');
	});

	it('выключенный отрезок дат забывает свои концы', () => {
		const on = { ...base, periodActive: true, periodFrom: '2026-08-01', periodTo: '2026-08-31' };
		const off = togglePeriod(on);
		expect(off).toMatchObject({ periodActive: false, periodFrom: '', periodTo: '' });
		expect(togglePeriod(off).periodActive).toBe(true);
	});
});

describe('строка «нашлось»', () => {
	it('перечисляет виды попаданий и склоняет их', () => {
		const hits = [hit('post', 'p1'), hit('comment', 'c1'), hit('comment', 'c2'), hit('day', 'd1')];
		expect(searchStats(hits)).toBe('1 запись, 2 комментария и 1 день');
	});

	it('одиннадцать — не «11 записи»', () => {
		const hits = Array.from({ length: 11 }, (_, i) => hit('post', `p${i}`));
		expect(searchStats(hits)).toBe('11 записей');
	});

	it('пусто — пустая строка', () => {
		expect(searchStats([])).toBe('');
	});
});

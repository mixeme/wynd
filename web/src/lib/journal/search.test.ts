import { describe, expect, it } from 'vitest';
import {
	highlightParts,
	localDayAfter,
	localDayStart,
	replaceSearchUrl,
	searchChipsFromParams,
	searchHref
} from './search';

describe('searchHref', () => {
	it('keeps the path when chips are empty', () => {
		expect(searchHref('/search', { q: '  ' })).toBe('/search');
	});

	it('passes query, period and media chips', () => {
		const href = searchHref('/search', {
			q: 'компот',
			periodActive: true,
			periodFrom: '2026-01-01',
			periodTo: '2026-02-01',
			hasPhoto: true,
			hasLocation: true
		});
		const url = new URL(href, 'https://wynd.local');
		expect(url.pathname).toBe('/search');
		expect(url.searchParams.get('q')).toBe('компот');
		expect(url.searchParams.get('from')).toBe('2026-01-01');
		expect(url.searchParams.get('to')).toBe('2026-02-01');
		expect(url.searchParams.get('photo')).toBe('1');
		expect(url.searchParams.get('location')).toBe('1');
	});

	it('omits dates when period chip is off', () => {
		const href = searchHref('/circles/c1/search', {
			q: 'а',
			periodActive: false,
			periodFrom: '2026-01-01',
			hasPhoto: true
		});
		const url = new URL(href, 'https://wynd.local');
		expect(url.pathname).toBe('/circles/c1/search');
		expect(url.searchParams.get('q')).toBe('а');
		expect(url.searchParams.get('photo')).toBe('1');
		expect(url.searchParams.get('from')).toBeNull();
	});
});

describe('searchChipsFromParams', () => {
	it('reads photo and period from the UI query', () => {
		const chips = searchChipsFromParams(new URLSearchParams('q=дом&from=2026-03-01&photo=1'));
		expect(chips).toEqual({
			q: 'дом',
			from: '2026-03-01',
			to: '',
			byDay: false,
			hasPhoto: true,
			hasLocation: false,
			author: '',
			circle: ''
		});
	});

	it('reads the day-based period from the query', () => {
		const chips = searchChipsFromParams(new URLSearchParams('from=2026-03-01&to=2026-03-05&day=1'));
		expect(chips).toMatchObject({ from: '2026-03-01', to: '2026-03-05', byDay: true });
		const href = searchHref('/search', {
			q: 'дом',
			periodActive: true,
			periodFrom: '2026-03-01',
			periodTo: '2026-03-05',
			byDay: true
		});
		expect(new URL(href, 'http://x').searchParams.get('day')).toBe('1');
	});

	it('the day after is the next local midnight', () => {
		const from = new Date(localDayStart('2026-11-01')!);
		const after = new Date(localDayAfter('2026-11-30')!);
		expect([from.getFullYear(), from.getMonth(), from.getDate(), from.getHours()]).toEqual([
			2026, 10, 1, 0
		]);
		expect([after.getFullYear(), after.getMonth(), after.getDate(), after.getHours()]).toEqual([
			2026, 11, 1, 0
		]);
		expect(after.getTime()).toBeGreaterThan(from.getTime());
	});

	it('keeps the circle the global search was opened from', () => {
		const href = searchHref('/search', { q: 'дом', circle: 'c1' });
		expect(searchChipsFromParams(new URL(href, 'http://x').searchParams).circle).toBe('c1');
	});

	it('reads author chip from the query', () => {
		const chips = searchChipsFromParams(new URLSearchParams('q=x&author=Аня'));
		expect(chips.author).toBe('Аня');
		const href = searchHref('/circles/c1/search', { q: 'x', author: 'Аня' });
		expect(new URL(href, 'https://wynd.local').searchParams.get('author')).toBe('Аня');
	});

	it('omits empty author from the query', () => {
		const href = searchHref('/circles/c1/search', { q: 'x', author: '  ' });
		expect(new URL(href, 'https://wynd.local').searchParams.get('author')).toBeNull();
	});
});

describe('replaceSearchUrl', () => {
	it('обновляет адрес записи истории вместе с sveltekit:pageurl', () => {
		history.replaceState(
			{
				'sveltekit:history': 7,
				'sveltekit:navigation': 6,
				'sveltekit:pageurl': 'http://localhost/search',
				'sveltekit:states': {}
			},
			'',
			'/search'
		);
		replaceSearchUrl('/search', { q: 'Ольга', hasPhoto: true });
		expect(location.pathname + location.search).toBe(
			'/search?q=%D0%9E%D0%BB%D1%8C%D0%B3%D0%B0&photo=1'
		);
		const state = history.state as Record<string, unknown>;
		// SvelteKit на шаге назад берёт адрес отсюда — без обновления «Назад»
		// возвращал поиск без запроса и фильтров (замечание владельца 27.11).
		expect(state['sveltekit:pageurl']).toBe(new URL(location.href).href);
		expect(state['sveltekit:history']).toBe(7);
		expect(state['sveltekit:navigation']).toBe(6);
	});
});

describe('highlightParts', () => {
	it('найденное слово — жирным как написано, без учёта регистра', () => {
		expect(highlightParts('Были на Даче в субботу', 'дач')).toEqual([
			{ text: 'Были на ', hit: false },
			{ text: 'Дач', hit: true },
			{ text: 'е в субботу', hit: false }
		]);
	});

	it('слова запроса выделяются по отдельности, в порядке текста', () => {
		expect(highlightParts('море и дача', 'дача море')).toEqual([
			{ text: 'море', hit: true },
			{ text: ' и ', hit: false },
			{ text: 'дача', hit: true }
		]);
	});

	it('пересекающиеся слова — один кусок', () => {
		expect(highlightParts('дачник', 'дач ник')).toEqual([{ text: 'дачник', hit: true }]);
	});

	it('без запроса или без совпадения — текст как есть', () => {
		expect(highlightParts('дача', '')).toEqual([{ text: 'дача', hit: false }]);
		expect(highlightParts('дача', 'лес')).toEqual([{ text: 'дача', hit: false }]);
	});

	it('спецсимволы запроса не ломают разбор', () => {
		expect(highlightParts('смета (2026)', '(2026)')).toEqual([
			{ text: 'смета ', hit: false },
			{ text: '(2026)', hit: true }
		]);
	});
});

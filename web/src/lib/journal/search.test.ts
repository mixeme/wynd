import { describe, expect, it } from 'vitest';
import { searchChipsFromParams, searchHref } from './search';

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
		const chips = searchChipsFromParams(
			new URLSearchParams('q=дом&from=2026-03-01&photo=1')
		);
		expect(chips).toEqual({
			q: 'дом',
			from: '2026-03-01',
			to: '',
			hasPhoto: true,
			hasLocation: false
		});
	});
});

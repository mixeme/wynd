import { beforeEach, expect, it } from 'vitest';
import {
	noteArrival,
	rememberSearchCircle,
	searchCircleSeed,
	searchFocusAfterNavigation,
	trackSearchScope
} from './search-scope';

const host = 'https://wynd.example';
const circleSearch = new URL('/circles/one/search?from=2026-11-20', host);
const globalSearch = new URL('/search?circle=one&from=2026-11-20', host);
const seed = {
	origin: '',
	id: 'one',
	name: 'Семья',
	color: 'indigo' as const,
	lastReadSeq: 21,
	canWrite: true,
	identityName: 'Аня'
};

beforeEach(() => {
	trackSearchScope(undefined, undefined);
	noteArrival('goto');
});

it('сохраняет оболочку круга и не поднимает клавиатуру при переключении области', () => {
	trackSearchScope(circleSearch, globalSearch);
	rememberSearchCircle(seed);
	expect(searchFocusAfterNavigation(globalSearch.pathname)).toBe(false);

	trackSearchScope(globalSearch, circleSearch);
	expect(searchCircleSeed(circleSearch.pathname, 'one')).toEqual(seed);
	expect(searchFocusAfterNavigation(circleSearch.pathname)).toBe(false);
});

it('обычный вход в поиск фокусирует поле и не подставляет сведения о предыдущем круге', () => {
	rememberSearchCircle(seed);
	trackSearchScope(globalSearch, new URL('/circles/two/search', host));
	expect(searchFocusAfterNavigation('/circles/two/search')).toBe(true);
	expect(searchCircleSeed('/circles/two/search', 'two')).toBeUndefined();
	trackSearchScope(globalSearch, circleSearch);
	expect(searchCircleSeed(circleSearch.pathname, 'one')).toBeUndefined();
});

it('возврат по истории не поднимает клавиатуру заново', () => {
	// «Назад» с записи на поиск: шаг по истории, строки уже были на экране.
	noteArrival('popstate');
	expect(searchFocusAfterNavigation(circleSearch.pathname)).toBe(false);
	// Обычный вход после возврата снова фокусирует поле.
	noteArrival('goto');
	expect(searchFocusAfterNavigation(circleSearch.pathname)).toBe(true);
});

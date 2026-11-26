import { describe, expect, it, vi } from 'vitest';
import { initLayoutMode, layoutMode, railLayout, splitLayout } from './layout.svelte';

const SPLIT_QUERY = '(min-width: 720px)';
const FULL_QUERY = '(min-width: 1060px)';

/** matchMedia с отдельными ответами на пороги 720 и 1000. */
function installWidths(split: boolean, full: boolean) {
	const queries = new Map<string, EventTarget & { matches: boolean; media: string }>();
	const make = (query: string, matches: boolean) => {
		const mq = new EventTarget() as EventTarget & { matches: boolean; media: string };
		mq.matches = matches;
		mq.media = query;
		queries.set(query, mq);
		return mq;
	};
	const fn = vi.fn((query: string) => {
		if (!queries.has(query)) make(query, query === SPLIT_QUERY ? split : full);
		return queries.get(query)!;
	});
	vi.stubGlobal('matchMedia', fn);
	Object.defineProperty(window, 'matchMedia', { writable: true, configurable: true, value: fn });
	return {
		set(nextSplit: boolean, nextFull: boolean) {
			queries.get(SPLIT_QUERY)!.matches = nextSplit;
			queries.get(FULL_QUERY)!.matches = nextFull;
			queries.get(SPLIT_QUERY)!.dispatchEvent(new Event('change'));
			queries.get(FULL_QUERY)!.dispatchEvent(new Event('change'));
		}
	};
}

// План 75: телефон — одна колонка, 720–999 — рейл аватаров, от 1000 — полный
// список и круг. Смена ширины видна сразу, после остановки слушателя — нет.
describe('layout mode', () => {
	it('follows the window width and stops listening', () => {
		const width = installWidths(false, false);
		const stop = initLayoutMode();

		expect(layoutMode()).toBe('phone');
		expect(splitLayout()).toBe(false);
		expect(railLayout()).toBe(false);

		width.set(true, false);
		expect(layoutMode()).toBe('rail');
		expect(splitLayout()).toBe(true);
		expect(railLayout()).toBe(true);

		width.set(true, true);
		expect(layoutMode()).toBe('full');
		expect(splitLayout()).toBe(true);
		expect(railLayout()).toBe(false);

		stop();
		width.set(false, false);
		expect(layoutMode()).toBe('full');
	});
});

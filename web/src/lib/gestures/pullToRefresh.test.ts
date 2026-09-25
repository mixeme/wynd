import { describe, expect, it } from 'vitest';
import {
	PTR,
	pullEnd,
	pullHeight,
	pullIdle,
	pullMarkHeight,
	pullMove,
	pullSettled,
	pullStart,
	pullVisible
} from './pullToRefresh';

describe('потянуть, чтобы обновить', () => {
	it('тянется только от самого верха списка', () => {
		const started = pullStart(pullIdle(), 100, 40);
		expect(started.phase).toBe('idle');
		expect(pullMove(started, 200, 40).pull).toBe(0);
	});

	it('ведёт полосу за пальцем и не пускает вверх', () => {
		let s = pullStart(pullIdle(), 100, 0);
		s = pullMove(s, 130, 0);
		expect(s.pull).toBe(30);
		s = pullMove(s, 60, 0);
		expect(s.pull).toBe(0);
	});

	it('дальше потолка не тянет', () => {
		let s = pullStart(pullIdle(), 0, 0);
		s = pullMove(s, 500, 0);
		expect(s.pull).toBe(PTR.maxPull);
	});

	it('отпустили не дотянув — всё возвращается', () => {
		let s = pullStart(pullIdle(), 0, 0);
		s = pullMove(s, PTR.threshold - 5, 0);
		s = pullEnd(s);
		expect(s).toEqual(pullIdle());
		expect(pullVisible(s)).toBe(false);
	});

	it('отпустили за чертой — полоса замирает и ждёт паузу', () => {
		let s = pullStart(pullIdle(), 0, 0);
		s = pullMove(s, PTR.threshold + 10, 0);
		s = pullEnd(s);
		expect(s.phase).toBe('settling');
		expect(pullHeight(s)).toBe(PTR.restHeight);
		s = pullSettled(s);
		expect(s.phase).toBe('refreshing');
	});

	it('во время обновления жест не начинается заново', () => {
		let s = pullSettled(pullEnd(pullMove(pullStart(pullIdle(), 0, 0), 80, 0)));
		expect(s.phase).toBe('refreshing');
		const again = pullStart(s, 0, 0);
		expect(again.phase).toBe('refreshing');
		expect(pullMove(again, 300, 0).pull).toBe(PTR.restHeight);
	});

	it('знак подрастает вместе с жестом, но не меньше восьми', () => {
		let s = pullStart(pullIdle(), 0, 0);
		s = pullMove(s, 4, 0);
		expect(pullMarkHeight(s)).toBe(8);
		s = pullMove(s, 40, 0);
		expect(pullMarkHeight(s)).toBe(34);
	});
});

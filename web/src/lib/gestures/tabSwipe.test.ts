import { describe, expect, it } from 'vitest';
import { SWIPE, swipeEnd, swipeMove, swipeOffset, swipeStart } from './tabSwipe';

const W = 390;

describe('tabSwipe', () => {
	it('переключает, если отпустить дальше трети ширины', () => {
		const s = swipeMove(swipeStart(300, 400, W, false), 300 - W / 2, 410);
		expect(s.phase).toBe('horizontal');
		expect(swipeEnd(s, true, true)).toBe(1);
		const back = swipeMove(swipeStart(100, 400, W, false), 100 + W / 2, 405);
		expect(swipeEnd(back, true, true)).toBe(-1);
	});

	it('возвращает, если не дотянул', () => {
		const s = swipeMove(swipeStart(300, 400, W, false), 300 - W / 5, 400);
		expect(swipeEnd(s, true, true)).toBe(0);
	});

	it('не трогает вертикальную прокрутку', () => {
		const s = swipeMove(swipeStart(200, 400, W, false), 185, 300);
		expect(s.phase).toBe('ignored');
		expect(swipeEnd(swipeMove(s, 0, 300), true, true)).toBe(0);
	});

	it('не переключает за крайнюю вкладку и тянет с сопротивлением', () => {
		const s = swipeMove(swipeStart(100, 400, W, false), 100 + W / 2, 400);
		expect(swipeEnd(s, false, true)).toBe(0);
		expect(swipeOffset(s, false, true)).toBe(W / 2 / 4);
	});

	it('на «Карте» — только от края', () => {
		expect(swipeStart(200, 400, W, true).phase).toBe('ignored');
		expect(swipeStart(SWIPE.edge - 4, 400, W, true).phase).toBe('pending');
		expect(swipeStart(W - 4, 400, W, true).phase).toBe('pending');
	});
});

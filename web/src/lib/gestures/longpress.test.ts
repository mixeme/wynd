import { describe, expect, it } from 'vitest';
import { CLICK_TAIL_MS, markLongPress, swallowsClick } from './longpress';

describe('длинное нажатие и клик следом', () => {
	it('без длинного нажатия клик проходит', () => {
		expect(swallowsClick(null, 1000)).toBe(false);
	});

	it('клик сразу после длинного нажатия — хвост того же жеста', () => {
		const mark = markLongPress(1000);
		expect(swallowsClick(mark, 1000)).toBe(true);
		expect(swallowsClick(mark, 1000 + CLICK_TAIL_MS - 1)).toBe(true);
	});

	it('клик позже — обычное нажатие', () => {
		const mark = markLongPress(1000);
		expect(swallowsClick(mark, 1000 + CLICK_TAIL_MS)).toBe(false);
		expect(swallowsClick(mark, 5000)).toBe(false);
	});

	it('порядок pointerup и click ни на что не влияет', () => {
		// Раньше флаг гасили в pointerup через queueMicrotask, и если click
		// приходил после микрозадачи, меню открывалось и тут же закрывалось.
		const mark = markLongPress(1000);
		// «pointerup» ничего не делает…
		// …а click в пределах хвоста всё равно проглочен.
		expect(swallowsClick(mark, 1005)).toBe(true);
	});
});

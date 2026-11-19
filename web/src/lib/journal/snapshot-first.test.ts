import { describe, expect, it } from 'vitest';
import { paintFromSnapshots } from './snapshot-first';

describe('paintFromSnapshots (C100)', () => {
	it('рисует всё, что нашлось в снимках', async () => {
		const drawn: string[] = [];
		const painted = await paintFromSnapshots(
			() => false,
			async () => () => drawn.push('grid'),
			async () => () => drawn.push('days')
		);
		expect(painted).toBe(true);
		expect(drawn).toEqual(['grid', 'days']);
	});

	it('без снимков ничего не рисует и отдаёт false', async () => {
		const painted = await paintFromSnapshots(
			() => false,
			async () => undefined,
			async () => undefined
		);
		expect(painted).toBe(false);
	});

	it('рисует то, что есть, если часть снимков отсутствует', async () => {
		let grid = false;
		const painted = await paintFromSnapshots(
			() => false,
			async () => () => {
				grid = true;
			},
			async () => undefined
		);
		expect(painted).toBe(true);
		expect(grid).toBe(true);
	});

	it('поздний снимок не перетирает свежий ответ', async () => {
		let grid = false;
		let fresh = false;
		const flight = paintFromSnapshots(
			() => fresh,
			async () => () => {
				grid = true;
			}
		);
		fresh = true;
		expect(await flight).toBe(false);
		expect(grid).toBe(false);
	});

	it('ошибка чтения одного снимка не мешает остальным', async () => {
		let days = false;
		const painted = await paintFromSnapshots(
			() => false,
			async () => {
				throw new Error('indexeddb');
			},
			async () => () => {
				days = true;
			}
		);
		expect(painted).toBe(true);
		expect(days).toBe(true);
	});
});

import { describe, expect, it } from 'vitest';
import { mergePages, refetchOlder } from './pages';

describe('pages', () => {
	it('склеивает порции без повторов', () => {
		const key = (n: number) => String(n);
		expect(mergePages([5, 4, 3], [], key)).toEqual([5, 4, 3]);
		expect(mergePages([5, 4, 3], [3, 2, 1], key)).toEqual([5, 4, 3, 2, 1]);
	});

	it('перечитывает старшие от новой границы, пока не наберёт сколько было', async () => {
		const pages: Record<string, { items: number[]; next?: string }> = {
			a: { items: [3, 2], next: 'b' },
			b: { items: [1, 0] }
		};
		const got = await refetchOlder((before) => Promise.resolve(pages[before]), 'a', 3);
		expect(got.items).toEqual([3, 2, 1, 0]);
		expect(got.next).toBeUndefined();
		expect((await refetchOlder((b) => Promise.resolve(pages[b]), undefined, 3)).items).toEqual([]);
	});
});

import { createHash } from 'node:crypto';
import { describe, expect, it } from 'vitest';
import { sha256 } from './sha256';

const hex = (b: Uint8Array) => [...b].map((x) => x.toString(16).padStart(2, '0')).join('');

describe('sha256 без crypto.subtle', () => {
	it('совпадает с node:crypto на границах блоков', () => {
		// 55/56/64 — хвост в один или два блока; 1000 — несколько полных блоков.
		for (const n of [0, 1, 55, 56, 63, 64, 65, 119, 120, 128, 1000]) {
			const data = new Uint8Array(n).map((_, i) => (i * 31 + 7) & 0xff);
			expect(hex(sha256(data)), `n=${n}`).toBe(createHash('sha256').update(data).digest('hex'));
		}
	});

	it('считает только свой срез буфера', () => {
		const buf = new Uint8Array(100).fill(9);
		const view = buf.subarray(10, 20);
		expect(hex(sha256(view))).toBe(createHash('sha256').update(view).digest('hex'));
	});
});

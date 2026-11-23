import { createHash } from 'node:crypto';
import { describe, expect, it } from 'vitest';
import { createSha256, sha256, sha256Hex } from './sha256';

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

	it('совпадает при обновлении кусками', () => {
		const data = new Uint8Array(200_000).map((_, i) => (i * 17 + 3) & 0xff);
		const want = createHash('sha256').update(data).digest('hex');
		for (const step of [1, 7, 63, 64, 65, 333, 4096]) {
			const stream = createSha256();
			for (let at = 0; at < data.length; at += step) {
				stream.update(data.subarray(at, at + step));
			}
			expect(stream.hex(), `step=${step}`).toBe(want);
		}
	});

	it('крупный файл считает потоком, а не одним digest', async () => {
		const data = new Uint8Array(9 * 1024 * 1024).map((_, i) => (i * 29 + 11) & 0xff);
		expect(await sha256Hex(data.buffer)).toBe(createHash('sha256').update(data).digest('hex'));
	});
});

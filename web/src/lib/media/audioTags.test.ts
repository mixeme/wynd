import { describe, expect, it } from 'vitest';
import { imageDataLooksBlank } from './audioTags';

function fill(rgb: [number, number, number]): Uint8ClampedArray {
	const data = new Uint8ClampedArray(32 * 32 * 4);
	for (let i = 0; i < data.length; i += 4) {
		data[i] = rgb[0];
		data[i + 1] = rgb[1];
		data[i + 2] = rgb[2];
		data[i + 3] = 255;
	}
	return data;
}

describe('imageDataLooksBlank', () => {
	it('rejects a black frame', () => {
		expect(imageDataLooksBlank(fill([0, 0, 0]))).toBe(true);
		expect(imageDataLooksBlank(fill([6, 6, 6]))).toBe(true);
	});

	it('keeps a frame that has a picture', () => {
		expect(imageDataLooksBlank(fill([40, 90, 160]))).toBe(false);
	});
});

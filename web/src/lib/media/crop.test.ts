import { describe, expect, it, vi } from 'vitest';
import {
	AVATAR_JPEG_QUALITY,
	AVATAR_OUTPUT_PX,
	clampCropScale,
	clampCropTransform,
	cropSquareInImage,
	cropWindow,
	initialCropTransform,
	minCoverScale,
	pinchCropTransform,
	zoomCropAroundPoint
} from './crop';
import {
	coverRectFromTransform,
	coverRectStyle,
	isCoverRect,
	transformFromCoverRect
} from './crop';

const viewport = { width: 360, height: 520, cropDiameter: 280 };

describe('crop math', () => {
	it('centers the crop window in the viewport', () => {
		const win = cropWindow(viewport);
		expect(win.centerX).toBe(180);
		expect(win.centerY).toBe(260);
		expect(win.left).toBe(40);
		expect(win.top).toBe(120);
		expect(win.size).toBe(280);
	});

	it('starts wide photos scaled to cover the crop circle', () => {
		const t = initialCropTransform(1600, 900, viewport);
		expect(t.scale).toBeCloseTo(280 / 900, 5);
		expect(t.centerX).toBe(180);
		expect(t.centerY).toBe(260);
	});

	it('maps a centered crop back to the image center', () => {
		const t = initialCropTransform(800, 800, viewport);
		const square = cropSquareInImage(800, 800, viewport, t);
		expect(square.x).toBeCloseTo(0, 5);
		expect(square.y).toBeCloseTo(0, 5);
		expect(square.size).toBeCloseTo(800, 5);
	});

	it('follows horizontal pan into image coordinates', () => {
		const wide = { width: 360, height: 520, cropDiameter: 280 };
		const base = initialCropTransform(1600, 900, wide);
		const shifted = clampCropTransform(1600, 900, wide, {
			...base,
			centerX: base.centerX + 28
		});
		const fromBase = cropSquareInImage(1600, 900, wide, base);
		const fromShift = cropSquareInImage(1600, 900, wide, shifted);
		expect(fromShift.x - fromBase.x).toBeCloseTo(-28 / base.scale, 5);
	});

	it('clamps scale to cover and a zoom ceiling', () => {
		const min = minCoverScale(800, 600, viewport.cropDiameter);
		expect(clampCropScale(800, 600, viewport, min * 0.5)).toBeCloseTo(min, 5);
		expect(clampCropScale(800, 600, viewport, min * 10)).toBeCloseTo(min * 6, 5);
	});

	it('keeps the crop square inside the image when panned hard', () => {
		const t = initialCropTransform(800, 800, viewport);
		const pan = clampCropTransform(800, 800, viewport, { ...t, centerX: 9999, centerY: -4000 });
		const sq = cropSquareInImage(800, 800, viewport, pan);
		expect(sq.x).toBeGreaterThanOrEqual(-0.001);
		expect(sq.y).toBeGreaterThanOrEqual(-0.001);
		expect(sq.x + sq.size).toBeLessThanOrEqual(800.001);
		expect(sq.y + sq.size).toBeLessThanOrEqual(800.001);
	});
});

describe('zoomCropAroundPoint', () => {
	it('keeps the image center when zooming at the center', () => {
		const next = zoomCropAroundPoint({ scale: 1, centerX: 180, centerY: 260 }, 180, 260, 2);
		expect(next.scale).toBe(2);
		expect(next.centerX).toBeCloseTo(180);
		expect(next.centerY).toBeCloseTo(260);
	});

	it('holds the viewport point under the cursor', () => {
		const next = zoomCropAroundPoint({ scale: 1, centerX: 180, centerY: 260 }, 40, 120, 2);
		expect(next.centerX).toBeCloseTo(320);
		expect(next.centerY).toBeCloseTo(400);
	});
});

describe('pinchCropTransform', () => {
	const start = { scale: 2, centerX: 100, centerY: 100 };

	it('zooms around a stationary midpoint', () => {
		const next = pinchCropTransform(start, 50, 50, 80, 50, 50, 160);
		const held = zoomCropAroundPoint(start, 50, 50, 4);
		expect(next.scale).toBeCloseTo(4);
		expect(next.centerX).toBeCloseTo(held.centerX);
		expect(next.centerY).toBeCloseTo(held.centerY);
	});

	it('pans when the midpoint moves at the same distance', () => {
		const next = pinchCropTransform(start, 50, 50, 100, 60, 40, 100);
		expect(next.scale).toBe(2);
		expect(next.centerX).toBeCloseTo(110);
		expect(next.centerY).toBeCloseTo(90);
	});
});

describe('renderAvatarCrop', () => {
	it('exports avatar output size and JPEG quality', () => {
		expect(AVATAR_OUTPUT_PX).toBe(512);
		expect(AVATAR_JPEG_QUALITY).toBe(0.85);
	});

	it('draws the crop onto a 512 JPEG canvas', async () => {
		const { renderAvatarCrop } = await import('./crop');
		const drawn: number[] = [];
		const ctx = {
			drawImage: (...args: unknown[]) => {
				drawn.push(...args.slice(1).map(Number));
			}
		};
		const getContext = vi
			.spyOn(HTMLCanvasElement.prototype, 'getContext')
			.mockReturnValue(ctx as unknown as CanvasRenderingContext2D);
		const toBlob = vi
			.spyOn(HTMLCanvasElement.prototype, 'toBlob')
			.mockImplementation(function (this: HTMLCanvasElement, cb, type, quality) {
				expect(this.width).toBe(AVATAR_OUTPUT_PX);
				expect(this.height).toBe(AVATAR_OUTPUT_PX);
				expect(type).toBe('image/jpeg');
				expect(quality).toBe(AVATAR_JPEG_QUALITY);
				cb(new Blob([new Uint8Array([0xff, 0xd8, 0xff])], { type: 'image/jpeg' }));
			});

		const t = initialCropTransform(800, 800, viewport);
		const data = await renderAvatarCrop({ width: 800, height: 800 } as ImageBitmap, viewport, t);
		expect(drawn).toEqual([0, 0, 800, 800, 0, 0, 512, 512]);
		expect(data.byteLength).toBeGreaterThan(0);
		getContext.mockRestore();
		toBlob.mockRestore();
	});
});

describe('кадр обложки', () => {
	const vp = { width: 390, height: 600, cropDiameter: 300 };

	it('без сдвига — квадрат по центру широкого снимка', () => {
		const t = initialCropTransform(4000, 3000, vp);
		const r = coverRectFromTransform(4000, 3000, vp, t);
		expect(r.w).toBeCloseTo(0.75);
		expect(r.h).toBeCloseTo(1);
		expect(r.x).toBeCloseTo(0.125);
		expect(r.y).toBeCloseTo(0);
		expect(isCoverRect(r)).toBe(true);
	});

	it('кадр возвращается в окно тем же, каким его выбрали', () => {
		const rect = { x: 0.5, y: 0.1, w: 0.375, h: 0.5 };
		const t = transformFromCoverRect(4000, 3000, vp, rect);
		const back = coverRectFromTransform(4000, 3000, vp, t);
		expect(back.x).toBeCloseTo(rect.x);
		expect(back.y).toBeCloseTo(rect.y);
		expect(back.w).toBeCloseTo(rect.w);
	});

	it('плитка рисует кадр процентами, без пропорций снимка', () => {
		expect(coverRectStyle({ x: 0.25, y: 0, w: 0.5, h: 1 })).toEqual({
			width: '200%',
			height: '100%',
			left: '-50%',
			top: '0%'
		});
	});

	it('кадр за краем снимка или пустой — не кадр', () => {
		expect(isCoverRect({ x: 0.8, y: 0, w: 0.5, h: 1 })).toBe(false);
		expect(isCoverRect({ x: 0, y: 0, w: 0, h: 0 })).toBe(false);
		expect(isCoverRect(undefined)).toBe(false);
	});
});

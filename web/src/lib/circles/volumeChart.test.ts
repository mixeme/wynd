import { describe, expect, it } from 'vitest';
import {
	VOLUME_BAR_STEP,
	VOLUME_BAR_WIDTH,
	VOLUME_CHART_WIDTH,
	volumeBarStep,
	volumeBarX,
	volumeChartOffsetX
} from './volumeChart';

describe('volumeChart', () => {
	it('centers a sparse pack and does not stretch a single bar', () => {
		expect(volumeBarStep(1)).toBe(VOLUME_BAR_STEP);
		expect(volumeChartOffsetX(1)).toBe((VOLUME_CHART_WIDTH - VOLUME_BAR_WIDTH) / 2);
		expect(volumeBarX(0, 1)).toBe(volumeChartOffsetX(1));
	});

	it('keeps the natural step while the pack fits', () => {
		const n = 14;
		const packed = (n - 1) * VOLUME_BAR_STEP + VOLUME_BAR_WIDTH;
		expect(packed).toBeLessThanOrEqual(VOLUME_CHART_WIDTH);
		expect(volumeBarStep(n)).toBe(VOLUME_BAR_STEP);
		expect(volumeChartOffsetX(n)).toBe((VOLUME_CHART_WIDTH - packed) / 2);
	});

	it('compresses the step when months overflow the viewBox', () => {
		const n = 20;
		const step = volumeBarStep(n);
		expect(step).toBeLessThan(VOLUME_BAR_STEP);
		expect(volumeBarX(n - 1, n) + VOLUME_BAR_WIDTH).toBeCloseTo(VOLUME_CHART_WIDTH);
		expect(volumeChartOffsetX(n)).toBe(0);
	});
});

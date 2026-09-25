import { describe, expect, it } from 'vitest';
import { evenPx, isLargeVideo, targetVideoSize, videoFitsSettings } from './compress';

describe('targetVideoSize', () => {
	it('keeps 1080p landscape', () => {
		expect(targetVideoSize(1920, 1080, 1080)).toEqual({ width: 1920, height: 1080 });
	});

	it('keeps 1080p portrait', () => {
		expect(targetVideoSize(1080, 1920, 1080)).toEqual({ width: 1080, height: 1920 });
	});

	it('scales 4K landscape to 1080p', () => {
		expect(targetVideoSize(3840, 2160, 1080)).toEqual({ width: 1920, height: 1080 });
	});

	it('scales 4K portrait to 1080p', () => {
		expect(targetVideoSize(2160, 3840, 1080)).toEqual({ width: 1080, height: 1920 });
	});

	it('does not upscale 720p', () => {
		expect(targetVideoSize(1280, 720, 1080)).toEqual({ width: 1280, height: 720 });
	});

	it('rounds to even pixels', () => {
		expect(evenPx(1081)).toBe(1080);
		const size = targetVideoSize(1919, 1079, 720);
		expect(size.width % 2).toBe(0);
		expect(size.height % 2).toBe(0);
	});
});

describe('videoFitsSettings', () => {
	it('accepts 1080p already under the bitrate', () => {
		const duration = 10;
		const size = (4000 * 1000 * duration) / 8;
		expect(videoFitsSettings(1920, 1080, duration, size, 1080, 6000)).toBe(true);
	});

	it('rejects 4K even when the file is small', () => {
		expect(videoFitsSettings(3840, 2160, 4, 400_000, 1080, 6000)).toBe(false);
	});

	it('rejects 1080p that is far over the bitrate', () => {
		const duration = 10;
		const size = (20_000 * 1000 * duration) / 8;
		expect(videoFitsSettings(1920, 1080, duration, size, 1080, 6000)).toBe(false);
	});
});

describe('isLargeVideo', () => {
	it('flags long or heavy files', () => {
		expect(isLargeVideo(21 * 1024 * 1024)).toBe(true);
		expect(isLargeVideo(1_000_000, 45)).toBe(true);
		expect(isLargeVideo(1_000_000, 10)).toBe(false);
	});
});

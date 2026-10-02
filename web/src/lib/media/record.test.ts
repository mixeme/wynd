import { describe, expect, it } from 'vitest';
import {
	formatDuration,
	levelFromSamples,
	peaksFromLevels,
	pickRecorderType,
	recordingExtension
} from './record';

describe('record', () => {
	it('сжимает уровни до волны, беря максимум отрезка', () => {
		expect(peaksFromLevels([])).toEqual([]);
		expect(peaksFromLevels([10, 120, -5], 64)).toEqual([10, 100, 0]);
		const levels = Array.from({ length: 128 }, (_, i) => (i % 2 ? 80 : 0));
		const peaks = peaksFromLevels(levels, 64);
		expect(peaks).toHaveLength(64);
		expect(peaks.every((p) => p === 80)).toBe(true);
	});

	it('тишина — ноль, громкое — до сотни', () => {
		expect(levelFromSamples(new Uint8Array(32).fill(128))).toBe(0);
		const loud = Uint8Array.from({ length: 32 }, (_, i) => (i % 2 ? 255 : 0));
		expect(levelFromSamples(loud)).toBe(100);
	});

	it('выбирает контейнер, который браузер пишет', () => {
		expect(pickRecorderType('audio', (t) => t.startsWith('audio/webm'))).toBe('audio/webm;codecs=opus');
		expect(pickRecorderType('audio', () => true)).toBe('audio/webm;codecs=opus');
		expect(pickRecorderType('audio', (t) => t.startsWith('audio/mp4'))).toBe('audio/mp4;codecs=mp4a.40.2');
		expect(pickRecorderType('video', () => false)).toBe('');
		expect(recordingExtension('audio/mp4')).toBe('m4a');
		expect(recordingExtension('audio/webm;codecs=opus')).toBe('webm');
		expect(recordingExtension('video/mp4;codecs=avc1')).toBe('mp4');
	});

	it('подписывает время', () => {
		expect(formatDuration(0)).toBe('0:00');
		expect(formatDuration(48_400)).toBe('0:48');
		expect(formatDuration(725_000)).toBe('12:05');
	});
});

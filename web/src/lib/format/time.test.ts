import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	formatClock,
	formatDayCardSubtitle,
	formatDeadline,
	formatEntryDate,
	formatMonthYear,
	formatPostTime,
	pluralPosts
} from './time';

describe('formatPostTime', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		vi.setSystemTime(new Date('2026-09-02T14:30:00'));
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('shows today', () => {
		expect(formatPostTime('2026-09-02T10:15:00Z')).toMatch(/^сегодня,/);
	});

	it('shows yesterday', () => {
		expect(formatPostTime('2026-09-01T21:40:00')).toMatch(/^вчера,/);
	});

	it('uses entry date when it differs from created day', () => {
		const text = formatPostTime('2026-08-30T10:00:00', '2026-07-12');
		expect(text).toContain('июл');
	});
});

describe('formatClock', () => {
	it('shows hours and minutes only', () => {
		expect(formatClock('2026-09-02T14:20:00')).toMatch(/14:20/);
	});
});

describe('pluralPosts', () => {
	it('declines Russian post count', () => {
		expect(pluralPosts(1)).toBe('1 запись');
		expect(pluralPosts(2)).toBe('2 записи');
		expect(pluralPosts(5)).toBe('5 записей');
		expect(pluralPosts(21)).toBe('21 запись');
	});
});

describe('format helpers', () => {
	it('formats entry date and month year', () => {
		expect(formatEntryDate('2026-08-12')).toContain('12');
		expect(formatMonthYear('2026-08-30')).toContain('2026');
	});

	it('formats deadline and day card subtitle', () => {
		expect(formatDeadline('2026-09-15T00:00:00Z')).toMatch(/^до /);
		expect(formatDayCardSubtitle('2026-08-12', 4)).toContain('4 записи');
	});

	it('returns empty for blank dates', () => {
		expect(formatEntryDate('')).toBe('');
		expect(formatDeadline('')).toBe('');
	});
});

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	formatClock,
	formatDayCardSubtitle,
	formatDeadline,
	formatEntryDate,
	formatCutoffDate,
	formatMonthYear,
	formatPostTime,
	pluralFiles,
	pluralPhotos,
	pluralPosts,
	formatAdminDayYear
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

	it('shows when the post was entered, not its day (C49)', () => {
		const text = formatPostTime('2026-08-30T10:00:00');
		expect(text).toBe('30 августа, 10:00');
	});
});

describe('formatClock', () => {
	it('shows hours and minutes only', () => {
		expect(formatClock('2026-09-02T14:20:00')).toMatch(/14:20/);
	});
});

describe('pluralFiles', () => {
	it('declines Russian file count', () => {
		expect(pluralFiles(1)).toBe('1 файл');
		expect(pluralFiles(2)).toBe('2 файла');
		expect(pluralFiles(5)).toBe('5 файлов');
		expect(pluralFiles(21)).toBe('21 файл');
	});
});

describe('pluralPhotos', () => {
	it('declines Russian photo count', () => {
		expect(pluralPhotos(1)).toBe('1 фотография');
		expect(pluralPhotos(2)).toBe('2 фотографии');
		expect(pluralPhotos(12)).toBe('12 фотографий');
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

	it('adds the year to a cutoff from another year', () => {
		const now = new Date(2026, 9, 19);
		expect(formatCutoffDate('2026-06-30', now)).toBe('30 июня');
		expect(formatCutoffDate('2025-06-30', now)).toBe('30 июня 2025');
		expect(formatCutoffDate('', now)).toBe('');
	});

	it('returns empty for blank dates', () => {
		expect(formatEntryDate('')).toBe('');
		expect(formatEntryDate('undefined')).toBe('');
		expect(formatDeadline('')).toBe('');
	});
});

describe('formatAdminDayYear', () => {
	it('год без «г.»', () => {
		expect(formatAdminDayYear('2026-10-24T09:00:00')).toBe('24 октября 2026');
	});
});

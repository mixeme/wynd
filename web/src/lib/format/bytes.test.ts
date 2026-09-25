import { describe, expect, it } from 'vitest';
import { formatBytes } from './bytes';

describe('formatBytes', () => {
	it('formats zero', () => {
		expect(formatBytes(0)).toBe('0 Б');
	});

	it('formats bytes and kilobytes', () => {
		expect(formatBytes(512)).toBe('512 Б');
		expect(formatBytes(1536)).toBe('1,5 КБ');
	});

	it('formats megabytes', () => {
		expect(formatBytes(1_572_864)).toBe('1,5 МБ');
		expect(formatBytes(12_582_912)).toBe('12 МБ');
	});
});

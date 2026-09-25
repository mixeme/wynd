import { describe, expect, it } from 'vitest';
import { isPostArchiveLocked } from './archive';

describe('isPostArchiveLocked', () => {
	it('locks a post before UTC midnight of the cutoff', () => {
		expect(
			isPostArchiveLocked(true, '2026-08-05', '2026-08-04T23:59:59.000Z')
		).toBe(true);
		expect(
			isPostArchiveLocked(true, '2026-08-05', '2026-08-05T00:00:00.000Z')
		).toBe(false);
	});

	it('does not lock when the cycle is inactive or the date is missing', () => {
		expect(isPostArchiveLocked(false, '2026-08-05', '2026-08-01T00:00:00.000Z')).toBe(
			false
		);
		expect(isPostArchiveLocked(true, undefined, '2026-08-01T00:00:00.000Z')).toBe(
			false
		);
	});
});

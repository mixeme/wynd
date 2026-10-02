import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { readExif } from './exif';

// 16x16 JPEG carrying DateTimeOriginal 2026:08:14 09:21:33 and
// GPS 59°56'19.2"N 30°18'32.4"E. jsdom gives import.meta.url an http scheme,
// so resolve from the vitest root instead.
const fixture = readFileSync(resolve(process.cwd(), 'src/test/fixtures/exif-gps.jpg'));

function fixtureFile(): File {
	return new File([new Uint8Array(fixture)], 'exif-gps.jpg', { type: 'image/jpeg' });
}

describe('readExif', () => {
	it('reads DateTimeOriginal into captured_at and entry_date', async () => {
		const hints = await readExif(fixtureFile());
		expect(hints.captured_at).toBe('2026-08-14T06:21:33.000Z');
		expect(hints.entry_date).toBe('2026-08-14');
	});

	// Regression: exifr computes latitude/longitude rather than exposing them as
	// tags, so a `pick` list naming them silently returned a photo with no place.
	it('reads GPS into geo_lat and geo_lng', async () => {
		const hints = await readExif(fixtureFile());
		expect(hints.geo_lat).toBeCloseTo(59.9387, 3);
		expect(hints.geo_lng).toBeCloseTo(30.309, 3);
	});

	// Android вырезает место, оставляя GPS 0/0 (NaN) или нули: места нет, значок не горит.
	it('treats redacted GPS as no place', async () => {
		for (const name of ['exif-gps-redacted.jpg', 'exif-gps-zero.jpg']) {
			const bytes = readFileSync(resolve(process.cwd(), `src/test/fixtures/${name}`));
			const hints = await readExif(new File([new Uint8Array(bytes)], name, { type: 'image/jpeg' }));
			expect(hints.geo_lat).toBeUndefined();
			expect(hints.geo_lng).toBeUndefined();
		}
	});

	it('returns no hints for a file without EXIF', async () => {
		const plain = new File([new Uint8Array([1, 2, 3, 4])], 'plain.bin', {
			type: 'application/octet-stream'
		});
		await expect(readExif(plain)).resolves.toEqual({});
	});
});

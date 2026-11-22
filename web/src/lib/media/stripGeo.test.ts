import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import exifr from 'exifr';
import { readExif } from './exif';
import { stripGeoQueueFile, stripJpegGps } from './stripGeo';
import type { QueueFile } from '$lib/idb/db';

// C22: при выключенных «Местах со снимков» координаты не идут в круг и в
// байтах оригинала. Ожидаемый итог — как у Android, вырезающего место:
// значения GPS обнулены (0/0 → NaN), остальной EXIF и байты файла целы.
const fixture = readFileSync(resolve(process.cwd(), 'src/test/fixtures/exif-gps.jpg'));

function fixtureFile(): File {
	return new File([new Uint8Array(fixture)], 'exif-gps.jpg', { type: 'image/jpeg' });
}

// Границы APP1-сегмента с EXIF в JPEG — для проверки «снаружи не тронуто».
function exifSegmentRange(bytes: Uint8Array): [number, number] {
	expect(bytes[0]).toBe(0xff);
	expect(bytes[1]).toBe(0xd8);
	const dv = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
	let at = 2;
	while (at + 4 <= bytes.length) {
		expect(bytes[at]).toBe(0xff);
		const marker = bytes[at + 1];
		if (marker === 0xda) break;
		const len = dv.getUint16(at + 2);
		if (marker === 0xe1 && bytes[at + 4] === 0x45) return [at, at + 2 + len];
		at += 2 + len;
	}
	throw new Error('APP1 с EXIF не найден');
}

describe('stripJpegGps', () => {
	it('strips GPS but keeps DateTimeOriginal', async () => {
		const bytes = new Uint8Array(fixture);
		const stripped = stripJpegGps(bytes);
		expect(stripped).toBeDefined();
		const data = await exifr.parse(new Blob([stripped!]), { exif: true, gps: true });
		const lat = data?.latitude;
		const lng = data?.longitude;
		// 0/0 и нули — «места нет»: NaN или нули «острова Нуль».
		expect(lat === undefined || Number.isNaN(lat) || lat === 0).toBe(true);
		expect(lng === undefined || Number.isNaN(lng) || lng === 0).toBe(true);
		expect(data?.DateTimeOriginal).toBeInstanceOf(Date);
	});

	it('readExif loses geo but keeps the date', async () => {
		const stripped = stripJpegGps(new Uint8Array(fixture))!;
		const hints = await readExif(new File([stripped], 'exif-gps.jpg', { type: 'image/jpeg' }));
		expect(hints.geo_lat).toBeUndefined();
		expect(hints.geo_lng).toBeUndefined();
		expect(hints.entry_date).toBe('2026-08-14');
	});

	it('keeps the length and touches bytes only inside the EXIF segment', () => {
		const bytes = new Uint8Array(fixture);
		const stripped = stripJpegGps(bytes)!;
		expect(stripped.length).toBe(bytes.length);
		const [from, to] = exifSegmentRange(bytes);
		let touched = 0;
		for (let i = 0; i < bytes.length; i++) {
			if (bytes[i] !== stripped[i]) {
				expect(i).toBeGreaterThanOrEqual(from);
				expect(i).toBeLessThan(to);
				touched += 1;
			}
		}
		expect(touched).toBeGreaterThan(0);
	});

	it('is idempotent: stripping twice changes nothing more', () => {
		const once = stripJpegGps(new Uint8Array(fixture))!;
		const twice = stripJpegGps(once)!;
		expect(Array.from(twice)).toEqual(Array.from(once));
	});

	it('leaves files without GPS pointer untouched', () => {
		// Минимальный JPEG: SOI, APP1 «Exif» без GPS-указателя (IFD0 пуст),
		// SOS и конец — GPS-блока нет, файл уходит как есть.
		const tiff = new Uint8Array([0x49, 0x49, 42, 0, 8, 0, 0, 0, 0, 0]);
		const payload = new Uint8Array([0x45, 0x78, 0x69, 0x66, 0, 0, ...tiff]);
		const seg = new Uint8Array(4 + payload.length);
		seg[0] = 0xff;
		seg[1] = 0xe1;
		seg[2] = (payload.length + 2) >> 8;
		seg[3] = (payload.length + 2) & 0xff;
		seg.set(payload, 4);
		const jpeg = new Uint8Array([0xff, 0xd8, ...seg, 0xff, 0xda, 0x00, 0x02, 0xff, 0xd9]);
		expect(stripJpegGps(jpeg)).toBeUndefined();
	});

	it('leaves non-JPEG and broken streams untouched', () => {
		expect(stripJpegGps(new Uint8Array([1, 2, 3, 4]))).toBeUndefined();
		expect(stripJpegGps(new Uint8Array([0xff, 0xd8, 0x00, 0x01]))).toBeUndefined();
	});
});

describe('stripGeoQueueFile', () => {
	function queueFile(bytes: Uint8Array, name = 'exif-gps.jpg', type = 'image/jpeg'): QueueFile {
		return { name, type, size: bytes.length, data: bytes.slice().buffer as ArrayBuffer };
	}

	it('replaces data of a JPEG with GPS by stripped bytes of the same size', () => {
		const out = stripGeoQueueFile(queueFile(new Uint8Array(fixture)));
		expect(out.size).toBe(fixture.length);
		expect(out.data.byteLength).toBe(fixture.length);
		expect(Array.from(new Uint8Array(out.data))).not.toEqual(Array.from(fixture));
	});

	it('passes through files that are not JPEG originals', () => {
		const q = queueFile(new Uint8Array(fixture), 'shot.png', 'image/png');
		expect(stripGeoQueueFile(q)).toBe(q);
		const noGps = queueFile(new Uint8Array([0xff, 0xd8, 0xff, 0xd9]), 'no.jpg');
		expect(stripGeoQueueFile(noGps)).toBe(noGps);
	});
});

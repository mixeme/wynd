import type { QueueFile } from '$lib/idb/db';

// C22: «Место со снимков» выключено — координаты не идут в круг и в байтах.
// Оригинал («Файл» и всё, что уходит без перекодирования) несёт GPS в EXIF;
// сжатые снимки и ролики его не хранят (перекодировка). Здесь GPS-значения
// JPEG обнуляются до отправки — по прецеденту Android: система, вырезая
// место из снимка, оставляет поля GPS с делением 0/0 (см. exif.ts), и exifr
// отдаёт NaN. Структура EXIF не трогается: даты и прочие поля остаются,
// сдвигов и перестройки байтов нет — файл той же длины.

const EXIF_POINTER_TAG = 0x8825; // IFD0 → GPS sub-IFD

// Размер значения одного элемента TIFF по типу; неизвестный тип — запрет.
const TYPE_SIZES: Record<number, number> = {
	1: 1, // BYTE
	2: 1, // ASCII
	3: 2, // SHORT
	4: 4, // LONG
	5: 8, // RATIONAL
	7: 1, // UNDEFINED
	9: 4, // SLONG
	10: 8 // SRATIONAL
};

/**
 * stripJpegGps — обнуляет значения всех элементов GPS-блока EXIF в JPEG.
 * Возвращает новые байты той же длины или undefined, если это не JPEG,
 * разметка не прочиталась или GPS в файле нет — в этих случаях файл
 * уходит как есть.
 */
export function stripJpegGps(bytes: Uint8Array): Uint8Array<ArrayBuffer> | undefined {
	if (bytes.length < 4 || bytes[0] !== 0xff || bytes[1] !== 0xd8) return undefined;
	const dv = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
	let at = 2;
	while (at + 4 <= bytes.length) {
		if (bytes[at] !== 0xff) return undefined;
		const marker = bytes[at + 1];
		if (marker === 0xff) {
			at += 1; // байт-заполнитель между сегментами
			continue;
		}
		if (marker === 0xda) break; // SOS: дальше сжатые данные без разметки
		if (marker === 0x01 || (marker >= 0xd0 && marker <= 0xd7)) {
			at += 2; // TEM и RST без длины
			continue;
		}
		const segLen = dv.getUint16(at + 2);
		if (segLen < 2 || at + 2 + segLen > bytes.length) return undefined;
		if (
			marker === 0xe1 && // APP1
			segLen >= 8 &&
			bytes[at + 4] === 0x45 && // "Exif\0\0"
			bytes[at + 5] === 0x78 &&
			bytes[at + 6] === 0x69 &&
			bytes[at + 7] === 0x66 &&
			bytes[at + 8] === 0x00 &&
			bytes[at + 9] === 0x00
		) {
			return zeroGpsIfd(bytes, at + 10, at + 2 + segLen);
		}
		at += 2 + segLen;
	}
	return undefined;
}

// zeroGpsIfd — обнуляет значения элементов GPS-подблока внутри TIFF-блока
// APP1-сегмента [tiffStart, tiffEnd). Без GPS-указателя в IFD0 — undefined.
function zeroGpsIfd(
	bytes: Uint8Array,
	tiffStart: number,
	tiffEnd: number
): Uint8Array<ArrayBuffer> | undefined {
	if (tiffStart + 8 > tiffEnd) return undefined;
	const dv = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
	const le = bytes[tiffStart] === 0x49; // "II" — little-endian; "MM" — big
	if (bytes[tiffStart] !== 0x49 && bytes[tiffStart] !== 0x4d) return undefined;
	if (dv.getUint16(tiffStart + 2, le) !== 42) return undefined;
	const ifd0 = tiffStart + dv.getUint32(tiffStart + 4, le);
	if (ifd0 + 2 > tiffEnd) return undefined;
	const n = dv.getUint16(ifd0, le);
	let gpsOffset = -1;
	for (let i = 0; i < n; i++) {
		const entry = ifd0 + 2 + i * 12;
		if (entry + 12 > tiffEnd) return undefined;
		if (dv.getUint16(entry, le) === EXIF_POINTER_TAG) {
			gpsOffset = dv.getUint32(entry + 8, le);
		}
	}
	if (gpsOffset <= 0 || tiffStart + gpsOffset + 2 > tiffEnd) return undefined;
	// Диапазоны значений для обнуления — в координатах всего файла.
	const ranges: Array<[number, number]> = [];
	const gps = tiffStart + gpsOffset;
	const gpsCount = dv.getUint16(gps, le);
	for (let i = 0; i < gpsCount; i++) {
		const entry = gps + 2 + i * 12;
		if (entry + 12 > tiffEnd) return undefined;
		const unit = TYPE_SIZES[dv.getUint16(entry + 2, le)];
		if (unit === undefined) return undefined;
		const count = dv.getUint32(entry + 4, le);
		const size = unit * count;
		if (size <= 0) continue;
		if (size <= 4) {
			ranges.push([entry + 8, entry + 8 + size]);
		} else {
			const value = dv.getUint32(entry + 8, le);
			if (value === 0 || tiffStart + value + size > tiffEnd) return undefined;
			ranges.push([tiffStart + value, tiffStart + value + size]);
		}
	}
	if (ranges.length === 0) return undefined;
	const out = new Uint8Array(bytes);
	for (const [from, to] of ranges) out.fill(0, from, to);
	return out;
}

/**
 * stripGeoQueueFile — для файла, идущего в круг без места (C22): у JPEG
 * обнуляет GPS в EXIF, остальное уходит как есть. Файл той же длины и
 * содержимого, кроме нулей вместо координат.
 */
export function stripGeoQueueFile(q: QueueFile): QueueFile {
	if (!/image\/jpe?g/i.test(q.type) && !/\.jpe?g$/i.test(q.name)) return q;
	const stripped = stripJpegGps(new Uint8Array(q.data));
	if (!stripped) return q;
	return { ...q, data: stripped.buffer as ArrayBuffer };
}

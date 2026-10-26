import type { ExifHints } from './exif';

/**
 * Дата съёмки ролика (A14, владелец 2026-10-23): камера телефона пишет её в
 * заголовок MP4 / MOV — `moov` → `mvhd`, секунды от 1904 года, UTC. Читаем
 * только заголовки блоков, а не файл целиком: `moov` бывает и в начале, и в
 * конце файла. Нет даты (ролик из браузера, WebM) — пусто: день остаётся
 * сегодняшним, как раньше.
 */

const EPOCH_1904_MS = Date.UTC(1904, 0, 1);
// Не больше стольких блоков верхнего уровня: битый файл не крутит цикл.
const MAX_BOXES = 64;

interface Box {
	type: string;
	start: number;
	headerSize: number;
	size: number;
}

async function readBox(file: Blob, at: number, limit: number): Promise<Box | undefined> {
	if (at + 8 > limit) return undefined;
	const head = new DataView(await file.slice(at, Math.min(at + 16, limit)).arrayBuffer());
	let size = head.getUint32(0);
	const type = String.fromCharCode(
		head.getUint8(4),
		head.getUint8(5),
		head.getUint8(6),
		head.getUint8(7)
	);
	let headerSize = 8;
	if (size === 1) {
		if (head.byteLength < 16) return undefined;
		size = Number(head.getBigUint64(8));
		headerSize = 16;
	} else if (size === 0) {
		size = limit - at;
	}
	if (size < headerSize || at + size > limit) return undefined;
	return { type, start: at, headerSize, size };
}

async function findChild(
	file: Blob,
	from: number,
	to: number,
	type: string
): Promise<Box | undefined> {
	let at = from;
	for (let i = 0; i < MAX_BOXES; i++) {
		const box = await readBox(file, at, to);
		if (!box) return undefined;
		if (box.type === type) return box;
		at += box.size;
	}
	return undefined;
}

/** Время создания из `mvhd`; 0 или нет блока — undefined. */
export async function mp4CreationTime(file: Blob): Promise<Date | undefined> {
	const moov = await findChild(file, 0, file.size, 'moov');
	if (!moov) return undefined;
	const mvhd = await findChild(file, moov.start + moov.headerSize, moov.start + moov.size, 'mvhd');
	if (!mvhd) return undefined;
	const body = new DataView(
		await file.slice(mvhd.start + mvhd.headerSize, mvhd.start + mvhd.headerSize + 12).arrayBuffer()
	);
	const version = body.getUint8(0);
	const seconds = version === 1 ? Number(body.getBigUint64(4)) : body.getUint32(4);
	if (!seconds) return undefined;
	return new Date(EPOCH_1904_MS + seconds * 1000);
}

/** Подсказка дня по ролику — того же вида, что `readExif` у снимка. */
export async function readVideoDate(file: Blob, now = new Date()): Promise<ExifHints> {
	try {
		const at = await mp4CreationTime(file);
		// Нули и чепуха в заголовке бывают: дату раньше 2000 года и из будущего
		// не берём.
		if (!at || at.getUTCFullYear() < 2000 || at.getTime() > now.getTime() + 86_400_000) return {};
		const y = at.getFullYear();
		const m = String(at.getMonth() + 1).padStart(2, '0');
		const d = String(at.getDate()).padStart(2, '0');
		return { captured_at: at.toISOString(), entry_date: `${y}-${m}-${d}` };
	} catch {
		return {};
	}
}

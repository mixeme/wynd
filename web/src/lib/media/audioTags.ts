import { isAudioMedia } from '$lib/journal/present';

// Обложка звука бывает и обложкой дня на всю ширину — 144 px там расплывались.
const COVER_EDGE = 1024;

export interface AudioTags {
	artist: string;
	title: string;
	cover?: ArrayBuffer;
}

function trimTag(value: string | undefined): string {
	const text = (value ?? '').trim();
	const chars = [...text];
	if (chars.length <= 200) return text;
	return chars.slice(0, 200).join('');
}

/** Тип для пустого или octet-stream, чтобы лента узнала звук без расширения. */
export function audioMime(name: string, type: string): string {
	const mime = type.toLowerCase().split(';')[0]?.trim() ?? '';
	if (mime.startsWith('audio/')) return type;
	const ext = name.split('.').pop()?.toLowerCase() ?? '';
	switch (ext) {
		case 'mp3':
			return 'audio/mpeg';
		case 'm4a':
		case 'aac':
			return 'audio/mp4';
		case 'ogg':
		case 'opus':
			return 'audio/ogg';
		case 'wav':
			return 'audio/wav';
		case 'flac':
			return 'audio/flac';
		default:
			return type || 'application/octet-stream';
	}
}

export function isAudioFile(file: File): boolean {
	return isAudioMedia(file.type, file.name);
}

async function coverJpeg(data: Uint8Array, mime: string): Promise<ArrayBuffer | undefined> {
	if (typeof document === 'undefined' || typeof createImageBitmap !== 'function') return;
	try {
		const bytes = new ArrayBuffer(data.byteLength);
		new Uint8Array(bytes).set(data);
		const blob = new Blob([bytes], { type: mime || 'image/jpeg' });
		const bitmap = await createImageBitmap(blob);
		const scale = Math.min(1, COVER_EDGE / Math.max(bitmap.width, bitmap.height));
		const width = Math.max(1, Math.round(bitmap.width * scale));
		const height = Math.max(1, Math.round(bitmap.height * scale));
		const canvas = document.createElement('canvas');
		canvas.width = width;
		canvas.height = height;
		const ctx = canvas.getContext('2d');
		if (!ctx) {
			bitmap.close();
			return;
		}
		ctx.drawImage(bitmap, 0, 0, width, height);
		bitmap.close();
		const jpeg = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/jpeg', 0.8));
		if (!jpeg) return;
		return jpeg.arrayBuffer();
	} catch {
		return;
	}
}

/**
 * Почти чёрный кадр: видеоэлемент часто отдаёт его, пока картинка ещё не
 * нарисована, и кодировщик в начале ролика тоже. Такой JPEG не годится в
 * обложку — лучше вовсе без кадра, и экран покажет ролик.
 */
export function imageDataLooksBlank(data: Uint8ClampedArray): boolean {
	const pixels = Math.floor(data.length / 4);
	if (pixels < 1) return true;
	let sum = 0;
	let max = 0;
	for (let i = 0; i < data.length; i += 4) {
		const y = data[i] * 0.2126 + data[i + 1] * 0.7152 + data[i + 2] * 0.0722;
		sum += y;
		if (y > max) max = y;
	}
	return sum / pixels < 8 && max < 18;
}

/** Картинка обложки — чёрный квадрат, её лучше заменить самим роликом. */
export async function coverImageIsBlank(url: string): Promise<boolean> {
	const img = new Image();
	img.src = url;
	try {
		await img.decode();
	} catch {
		return false;
	}
	if (img.naturalWidth < 1 || img.naturalHeight < 1) return false;
	const canvas = document.createElement('canvas');
	canvas.width = 32;
	canvas.height = 32;
	const ctx = canvas.getContext('2d', { willReadFrequently: true });
	if (!ctx) return false;
	ctx.drawImage(img, 0, 0, 32, 32);
	return imageDataLooksBlank(ctx.getImageData(0, 0, 32, 32).data);
}

/**
 * JPEG кадра ролика, длинная сторона до 1024 px. Кадр читается из файла, не
 * из видеоэлемента: тот до первой отрисовки даёт чёрный квадрат. Чёрный
 * кадр в начале пропускается. Не вышло — ролик уходит без картинки, и «Дни»
 * со «Сеткой» покажут кадр самим видео.
 */
export async function videoPosterJpeg(file: Blob): Promise<ArrayBuffer | undefined> {
	if (typeof document === 'undefined') return;
	try {
		const { ALL_FORMATS, BlobSource, Input, VideoSampleSink } = await import('mediabunny');
		const input = new Input({ source: new BlobSource(file), formats: ALL_FORMATS });
		try {
			const track = await input.getPrimaryVideoTrack();
			if (!track) return;
			const first = await track.getFirstTimestamp();
			const origin = Number.isFinite(first) ? first : 0;
			// Один декодер на весь проход и программный, не аппаратный: три
			// отдельных кадра занимали все декодеры телефона, и следующее
			// видео из галереи уже не сжималось.
			const sink = new VideoSampleSink(track, { hardwareAcceleration: 'prefer-software' });
			const probe = document.createElement('canvas');
			probe.width = 32;
			probe.height = 32;
			const probeCtx = probe.getContext('2d', { willReadFrequently: true });
			if (!probeCtx) return;
			const marks = [origin, origin + 0.5, origin + 1.5];
			let mark = 0;
			for await (const sample of sink.samples(origin, marks[marks.length - 1] + 0.05)) {
				try {
					if (sample.displayWidth < 1 || sample.displayHeight < 1) continue;
					if (sample.timestamp + 0.04 < marks[mark]) continue;
					while (mark < marks.length - 1 && sample.timestamp + 0.04 >= marks[mark + 1]) mark += 1;
					sample.draw(probeCtx, 0, 0, probe.width, probe.height);
					if (imageDataLooksBlank(probeCtx.getImageData(0, 0, probe.width, probe.height).data)) {
						mark += 1;
						if (mark >= marks.length) break;
						continue;
					}
					return await sampleToJpeg(sample);
				} finally {
					sample.close();
				}
			}
		} finally {
			input.dispose();
		}
	} catch {
		return;
	}
}

async function sampleToJpeg(sample: {
	displayWidth: number;
	displayHeight: number;
	draw(
		context: CanvasRenderingContext2D,
		dx: number,
		dy: number,
		dWidth?: number,
		dHeight?: number
	): void;
}): Promise<ArrayBuffer | undefined> {
	const scale = Math.min(1, COVER_EDGE / Math.max(sample.displayWidth, sample.displayHeight));
	const width = Math.max(1, Math.round(sample.displayWidth * scale));
	const height = Math.max(1, Math.round(sample.displayHeight * scale));
	const canvas = document.createElement('canvas');
	canvas.width = width;
	canvas.height = height;
	const ctx = canvas.getContext('2d');
	if (!ctx) return;
	sample.draw(ctx, 0, 0, width, height);
	const jpeg = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/jpeg', 0.8));
	if (!jpeg) return;
	return jpeg.arrayBuffer();
}

/** Теги читает отправитель. Ошибка разбора не мешает приложить файл как есть. */
export async function readAudioTags(file: File): Promise<AudioTags> {
	const empty: AudioTags = { artist: '', title: '' };
	if (!isAudioFile(file)) return empty;
	try {
		const { ALL_FORMATS, BlobSource, Input } = await import('mediabunny');
		const input = new Input({ source: new BlobSource(file), formats: ALL_FORMATS });
		try {
			const tags = await input.getMetadataTags();
			const image =
				tags.images?.find((item) => item.kind === 'coverFront') ?? tags.images?.[0];
			const cover = image ? await coverJpeg(image.data, image.mimeType) : undefined;
			return { artist: trimTag(tags.artist), title: trimTag(tags.title), cover };
		} finally {
			input.dispose();
		}
	} catch {
		return empty;
	}
}

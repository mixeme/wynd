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

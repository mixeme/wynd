import type { CompressionSettings } from '$lib/journal/types';

const DEFAULT_MAX_PX = 2048;
const DEFAULT_QUALITY = 0.85;

export interface CompressedImage {
	data: ArrayBuffer;
	type: string;
	name: string;
	size: number;
}

export async function compressImage(
	file: File,
	settings?: CompressionSettings
): Promise<CompressedImage> {
	const maxPx = settings?.photo_max_px || DEFAULT_MAX_PX;
	const quality = (settings?.photo_quality || Math.round(DEFAULT_QUALITY * 100)) / 100;

	const bitmap = await createImageBitmap(file);
	const scale = Math.min(1, maxPx / Math.max(bitmap.width, bitmap.height));
	const w = Math.round(bitmap.width * scale);
	const h = Math.round(bitmap.height * scale);

	const canvas = document.createElement('canvas');
	canvas.width = w;
	canvas.height = h;
	const ctx = canvas.getContext('2d');
	if (!ctx) throw new Error('canvas_unavailable');
	ctx.drawImage(bitmap, 0, 0, w, h);
	bitmap.close();

	const blob = await new Promise<Blob>((resolve, reject) => {
		canvas.toBlob(
			(b) => (b ? resolve(b) : reject(new Error('compress_failed'))),
			'image/jpeg',
			quality
		);
	});

	const buffer = await blob.arrayBuffer();
	const base = file.name.replace(/\.[^.]+$/, '') || 'photo';
	return {
		data: buffer,
		type: 'image/jpeg',
		name: `${base}.jpg`,
		size: buffer.byteLength
	};
}

export function isImageFile(file: File): boolean {
	return file.type.startsWith('image/');
}

export function isVideoFile(file: File): boolean {
	return file.type.startsWith('video/');
}

export async function fileToQueueBuffer(file: File): Promise<{
	data: ArrayBuffer;
	type: string;
	name: string;
	size: number;
}> {
	const buffer = await file.arrayBuffer();
	return { data: buffer, type: file.type, name: file.name, size: file.size };
}

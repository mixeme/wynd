import type { CompressionSettings } from '$lib/journal/types';

const DEFAULT_MAX_PX = 2048;
const DEFAULT_QUALITY = 80;
export const DEFAULT_VIDEO_MAX_P = 1080;
export const DEFAULT_VIDEO_BITRATE_KBPS = 6000;
const BITRATE_SLACK = 1.05;

export interface CompressedMedia {
	data: ArrayBuffer;
	type: string;
	name: string;
	size: number;
}

export function evenPx(n: number): number {
	const v = Math.round(n);
	return Math.max(2, v - (v % 2));
}

/** 1080p: the shorter side is at most `maxP` (landscape 1920×1080, portrait 1080×1920). */
export function targetVideoSize(
	width: number,
	height: number,
	maxP: number
): { width: number; height: number } {
	const short = Math.min(width, height);
	if (short <= 0 || maxP <= 0) {
		return { width: evenPx(width), height: evenPx(height) };
	}
	const scale = Math.min(1, maxP / short);
	return {
		width: evenPx(width * scale),
		height: evenPx(height * scale)
	};
}

export function videoFitsSettings(
	width: number,
	height: number,
	durationSec: number,
	sizeBytes: number,
	maxP: number,
	bitrateKbps: number
): boolean {
	const target = targetVideoSize(width, height, maxP);
	if (width > target.width || height > target.height) return false;
	if (!(durationSec > 0) || bitrateKbps <= 0) return false;
	const kbps = (sizeBytes * 8) / durationSec / 1000;
	return kbps <= bitrateKbps * BITRATE_SLACK;
}

export function isLargeVideo(sizeBytes: number, durationSec?: number): boolean {
	if (sizeBytes >= 20 * 1024 * 1024) return true;
	return (durationSec ?? 0) >= 45;
}

export async function compressImage(
	file: File,
	settings?: CompressionSettings
): Promise<CompressedMedia> {
	const maxPx = settings?.photo_max_px || DEFAULT_MAX_PX;
	const quality = (settings?.photo_quality || DEFAULT_QUALITY) / 100;

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

	const blob = await encodePhoto(canvas, quality);
	const buffer = await blob.arrayBuffer();
	const base = file.name.replace(/\.[^.]+$/, '') || 'photo';
	const ext = blob.type === 'image/webp' ? 'webp' : 'jpg';
	return {
		data: buffer,
		type: blob.type,
		name: `${base}.${ext}`,
		size: buffer.byteLength
	};
}

/** Холст, который умеет отдать картинку; у тестов — подделка. */
export interface BlobEncoder {
	toBlob(callback: BlobCallback, type?: string, quality?: number): void;
}

function toBlobAs(canvas: BlobEncoder, type: string, quality: number): Promise<Blob> {
	return new Promise<Blob>((resolve, reject) => {
		canvas.toBlob((b) => (b ? resolve(b) : reject(new Error('compress_failed'))), type, quality);
	});
}

/**
 * WebP, а без кодировщика WebP — JPEG. Браузер без WebP-кодировщика (Safari
 * до 16) молча отдаёт PNG: раньше он уходил на сервер с типом и расширением
 * WebP и размером несжатой картинки (план 42, MED-2).
 */
export async function encodePhoto(canvas: BlobEncoder, quality: number): Promise<Blob> {
	const webp = await toBlobAs(canvas, 'image/webp', quality);
	if (webp.type === 'image/webp') return webp;
	return toBlobAs(canvas, 'image/jpeg', quality);
}

export async function compressVideo(
	file: File,
	settings?: CompressionSettings,
	onProgress?: (progress: number) => void
): Promise<CompressedMedia> {
	try {
		const { encodeVideo } = await import('./video-encode');
		return await encodeVideo(file, settings, onProgress);
	} catch {
		return fileToQueueBuffer(file);
	}
}

export function isImageFile(file: File): boolean {
	return file.type.startsWith('image/');
}

export function isVideoFile(file: File): boolean {
	return file.type.startsWith('video/');
}

export async function fileToQueueBuffer(file: File): Promise<CompressedMedia> {
	const buffer = await file.arrayBuffer();
	return { data: buffer, type: file.type, name: file.name, size: file.size };
}

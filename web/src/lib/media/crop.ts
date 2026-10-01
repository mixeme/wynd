export const AVATAR_OUTPUT_PX = 512;
export const AVATAR_JPEG_QUALITY = 0.85;

export interface CropViewport {
	width: number;
	height: number;
	/** Diameter of the circular crop window in viewport pixels. */
	cropDiameter: number;
}

export interface CropTransform {
	/** Viewport pixels per one image pixel. */
	scale: number;
	/** Image center X in viewport coordinates. */
	centerX: number;
	/** Image center Y in viewport coordinates. */
	centerY: number;
}

export interface CroppedImage {
	data: ArrayBuffer;
	type: string;
	name: string;
	size: number;
}

export function cropWindow(viewport: CropViewport): {
	left: number;
	top: number;
	size: number;
	centerX: number;
	centerY: number;
} {
	const size = viewport.cropDiameter;
	const centerX = viewport.width / 2;
	const centerY = viewport.height / 2;
	return {
		left: centerX - size / 2,
		top: centerY - size / 2,
		size,
		centerX,
		centerY
	};
}

/** Minimum scale so the crop square is fully covered by the image. */
export function minCoverScale(
	imageWidth: number,
	imageHeight: number,
	cropDiameter: number
): number {
	return Math.max(cropDiameter / imageWidth, cropDiameter / imageHeight);
}

export function initialCropTransform(
	imageWidth: number,
	imageHeight: number,
	viewport: CropViewport
): CropTransform {
	const { centerX, centerY } = cropWindow(viewport);
	return {
		scale: minCoverScale(imageWidth, imageHeight, viewport.cropDiameter),
		centerX,
		centerY
	};
}

/** Map the on-screen crop square to a square region in image pixel space. */
export function cropSquareInImage(
	imageWidth: number,
	imageHeight: number,
	viewport: CropViewport,
	transform: CropTransform
): { x: number; y: number; size: number } {
	const { left, top, size: cropSize } = cropWindow(viewport);
	const { scale, centerX, centerY } = transform;
	const displayLeft = centerX - (imageWidth * scale) / 2;
	const displayTop = centerY - (imageHeight * scale) / 2;
	return {
		x: (left - displayLeft) / scale,
		y: (top - displayTop) / scale,
		size: cropSize / scale
	};
}

/**
 * Кадр обложки (4.16): квадрат снимка в долях его ширины и высоты. Новый
 * файл не пишется — лента рисует этот кусок того же снимка, альбом показывает
 * снимок целиком. Квадрат в пикселях, поэтому w·ширина = h·высота.
 */
export interface CoverRect {
	x: number;
	y: number;
	w: number;
	h: number;
}

const clamp01 = (v: number) => Math.min(1, Math.max(0, v));

export function coverRectFromTransform(
	imageWidth: number,
	imageHeight: number,
	viewport: CropViewport,
	transform: CropTransform
): CoverRect {
	const sq = cropSquareInImage(imageWidth, imageHeight, viewport, transform);
	const w = Math.min(1, sq.size / imageWidth);
	const h = Math.min(1, sq.size / imageHeight);
	return {
		x: clamp01(Math.min(sq.x / imageWidth, 1 - w)),
		y: clamp01(Math.min(sq.y / imageHeight, 1 - h)),
		w,
		h
	};
}

/** Обратное: каким был снимок в окне, когда кадр выбирали. */
export function transformFromCoverRect(
	imageWidth: number,
	imageHeight: number,
	viewport: CropViewport,
	rect: CoverRect
): CropTransform {
	const { centerX, centerY, size } = cropWindow(viewport);
	const scale = clampCropScale(imageWidth, imageHeight, viewport, size / (rect.w * imageWidth));
	const px = (rect.x + rect.w / 2) * imageWidth;
	const py = (rect.y + rect.h / 2) * imageHeight;
	return clampCropTransform(imageWidth, imageHeight, viewport, {
		scale,
		centerX: centerX - (px - imageWidth / 2) * scale,
		centerY: centerY - (py - imageHeight / 2) * scale
	});
}

/** Кадр в пределах снимка и не пустой — иначе лента режет по центру, как без него. */
export function isCoverRect(rect: Partial<CoverRect> | null | undefined): rect is CoverRect {
	if (!rect) return false;
	const { x, y, w, h } = rect;
	if (![x, y, w, h].every((v) => typeof v === 'number' && Number.isFinite(v))) return false;
	const eps = 1e-6;
	return x! >= -eps && y! >= -eps && w! > 0 && h! > 0 && x! + w! <= 1 + eps && y! + h! <= 1 + eps;
}

/**
 * Как нарисовать кадр в квадратной плитке: снимок растянут так, что кадр
 * занимает плитку целиком. Проценты от плитки — пропорции снимка не нужны.
 */
export function coverRectStyle(rect: CoverRect): {
	width: string;
	height: string;
	left: string;
	top: string;
} {
	const pct = (v: number) => `${Math.round(v * 10000) / 100}%`;
	return {
		width: pct(1 / rect.w),
		height: pct(1 / rect.h),
		left: pct(-rect.x / rect.w),
		top: pct(-rect.y / rect.h)
	};
}

export function clampCropTransform(
	imageWidth: number,
	imageHeight: number,
	viewport: CropViewport,
	transform: CropTransform
): CropTransform {
	const { left, top, size: cropSize } = cropWindow(viewport);
	const cropRight = left + cropSize;
	const cropBottom = top + cropSize;
	const { scale } = transform;
	const halfW = (imageWidth * scale) / 2;
	const halfH = (imageHeight * scale) / 2;

	let centerX = transform.centerX;
	let centerY = transform.centerY;

	const minX = cropRight - halfW;
	const maxX = left + halfW;
	if (minX <= maxX) {
		centerX = Math.min(maxX, Math.max(minX, centerX));
	} else {
		centerX = (minX + maxX) / 2;
	}

	const minY = cropBottom - halfH;
	const maxY = top + halfH;
	if (minY <= maxY) {
		centerY = Math.min(maxY, Math.max(minY, centerY));
	} else {
		centerY = (minY + maxY) / 2;
	}

	return { scale, centerX, centerY };
}

export function clampCropScale(
	imageWidth: number,
	imageHeight: number,
	viewport: CropViewport,
	scale: number
): number {
	const min = minCoverScale(imageWidth, imageHeight, viewport.cropDiameter);
	return Math.max(min, Math.min(min * 6, scale));
}

/** Zoom so the image point under (anchorX, anchorY) stays put. */
export function zoomCropAroundPoint(
	base: CropTransform,
	anchorX: number,
	anchorY: number,
	nextScale: number
): CropTransform {
	if (base.scale === 0) return { scale: nextScale, centerX: base.centerX, centerY: base.centerY };
	const ratio = nextScale / base.scale;
	return {
		scale: nextScale,
		centerX: base.centerX + (anchorX - base.centerX) * (1 - ratio),
		centerY: base.centerY + (anchorY - base.centerY) * (1 - ratio)
	};
}

/**
 * Pinch: zoom around the current midpoint of two touches, including two-finger pan
 * from the start midpoint.
 */
export function pinchCropTransform(
	start: CropTransform,
	startMidX: number,
	startMidY: number,
	startDistance: number,
	nowMidX: number,
	nowMidY: number,
	nowDistance: number
): CropTransform {
	const nextScale = startDistance === 0 ? start.scale : start.scale * (nowDistance / startDistance);
	const panned: CropTransform = {
		scale: start.scale,
		centerX: start.centerX + (nowMidX - startMidX),
		centerY: start.centerY + (nowMidY - startMidY)
	};
	return zoomCropAroundPoint(panned, nowMidX, nowMidY, nextScale);
}

export async function decodeAvatarBitmap(source: ImageBitmapSource): Promise<ImageBitmap> {
	if (typeof HTMLImageElement !== 'undefined' && source instanceof HTMLImageElement) {
		return createImageBitmap(source);
	}
	try {
		return await createImageBitmap(source, { imageOrientation: 'from-image' });
	} catch {
		return await createImageBitmap(source);
	}
}

export async function renderAvatarCrop(
	bitmap: ImageBitmap,
	viewport: CropViewport,
	transform: CropTransform
): Promise<ArrayBuffer> {
	const clamped = clampCropTransform(bitmap.width, bitmap.height, viewport, transform);
	const { x, y, size } = cropSquareInImage(bitmap.width, bitmap.height, viewport, clamped);

	const canvas = document.createElement('canvas');
	canvas.width = AVATAR_OUTPUT_PX;
	canvas.height = AVATAR_OUTPUT_PX;
	const ctx = canvas.getContext('2d');
	if (!ctx) throw new Error('canvas_unavailable');

	ctx.drawImage(bitmap, x, y, size, size, 0, 0, AVATAR_OUTPUT_PX, AVATAR_OUTPUT_PX);

	const blob = await new Promise<Blob>((resolve, reject) => {
		canvas.toBlob(
			(b) => (b ? resolve(b) : reject(new Error('crop_failed'))),
			'image/jpeg',
			AVATAR_JPEG_QUALITY
		);
	});
	return blob.arrayBuffer();
}

export async function cropAvatarBitmap(
	bitmap: ImageBitmap,
	viewport: CropViewport,
	transform: CropTransform,
	name = 'avatar'
): Promise<CroppedImage> {
	const data = await renderAvatarCrop(bitmap, viewport, transform);
	return {
		data,
		type: 'image/jpeg',
		name: `${name.replace(/\.[^.]+$/, '') || 'avatar'}.jpg`,
		size: data.byteLength
	};
}

export async function cropAvatarFile(
	file: File,
	viewport: CropViewport,
	transform: CropTransform
): Promise<CroppedImage> {
	const bitmap = await decodeAvatarBitmap(file);
	try {
		return await cropAvatarBitmap(bitmap, viewport, transform, file.name);
	} finally {
		bitmap.close();
	}
}

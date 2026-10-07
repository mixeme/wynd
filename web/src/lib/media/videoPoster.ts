import { localPosterId } from '$lib/journal/present';
import { getMediaUrl, saveLocalPoster } from './objectUrl';

// Кадр бывает и обложкой дня на всю ширину — как обложка звука.
const POSTER_EDGE = 1024;
// Начало ролика часто чёрное (затемнение, камера ещё не открылась) —
// тогда берём кадр подальше.
const MARKS = [0.1, 1, 2.5];

/** Почти чёрный кадр: в обложку не годится. */
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

function waitFor(video: HTMLVideoElement, event: string, ms: number): Promise<boolean> {
	return new Promise((resolve) => {
		const done = (ok: boolean) => {
			clearTimeout(timer);
			video.removeEventListener(event, onOk);
			video.removeEventListener('error', onFail);
			resolve(ok);
		};
		const onOk = () => done(true);
		const onFail = () => done(false);
		const timer = setTimeout(() => done(false), ms);
		video.addEventListener(event, onOk);
		video.addEventListener('error', onFail);
	});
}

/** Ждём, пока браузер нарисует кадр: `seeked` приходит раньше картинки. */
function painted(video: HTMLVideoElement, ms: number): Promise<void> {
	return new Promise((resolve) => {
		const timer = setTimeout(resolve, ms);
		if ('requestVideoFrameCallback' in video) {
			video.requestVideoFrameCallback(() => {
				clearTimeout(timer);
				resolve();
			});
		}
	});
}

function frameIsBlank(video: HTMLVideoElement, probe: CanvasRenderingContext2D): boolean {
	probe.drawImage(video, 0, 0, 32, 32);
	return imageDataLooksBlank(probe.getImageData(0, 0, 32, 32).data);
}

async function frameJpeg(video: HTMLVideoElement): Promise<ArrayBuffer | undefined> {
	const scale = Math.min(1, POSTER_EDGE / Math.max(video.videoWidth, video.videoHeight));
	const canvas = document.createElement('canvas');
	canvas.width = Math.max(1, Math.round(video.videoWidth * scale));
	canvas.height = Math.max(1, Math.round(video.videoHeight * scale));
	const ctx = canvas.getContext('2d');
	if (!ctx) return;
	ctx.drawImage(video, 0, 0, canvas.width, canvas.height);
	const jpeg = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, 'image/jpeg', 0.8));
	return jpeg?.arrayBuffer();
}

/**
 * JPEG кадра ролика, длинная сторона до 1024 px. Кадр снимает обычный
 * видеоэлемент: декодер WebCodecs есть не везде (Firefox на Android), а
 * играть ролик умеет любой браузер. Не вышло — ролик уходит без кадра, и
 * плитка рисует заглушку.
 */
export async function videoPosterJpeg(source: Blob | string): Promise<ArrayBuffer | undefined> {
	if (typeof document === 'undefined') return;
	const url = typeof source === 'string' ? source : URL.createObjectURL(source);
	const video = document.createElement('video');
	video.muted = true;
	video.playsInline = true;
	video.preload = 'auto';
	// В документе и почти невидимо: оторванный от страницы элемент телефон
	// может не декодировать вовсе.
	video.style.cssText =
		'position:fixed;left:0;top:0;width:2px;height:2px;opacity:.01;pointer-events:none';
	document.body.appendChild(video);
	try {
		video.src = url;
		if (video.readyState < 2 && !(await waitFor(video, 'loadeddata', 10000))) return;
		if (!video.videoWidth || !video.videoHeight) return;
		const probe = document.createElement('canvas');
		probe.width = 32;
		probe.height = 32;
		const probeCtx = probe.getContext('2d', { willReadFrequently: true });
		if (!probeCtx) return;

		// У записи с камеры длительность бывает неизвестна (Infinity).
		const duration = Number.isFinite(video.duration) ? video.duration : 0;
		for (const mark of MARKS) {
			if (duration > 0 && mark >= duration && mark !== MARKS[0]) break;
			video.currentTime = duration > 0 ? Math.min(mark, duration / 2) : mark;
			if (!(await waitFor(video, 'seeked', 3000))) break;
			await painted(video, 250);
			if (!frameIsBlank(video, probeCtx)) return await frameJpeg(video);
		}

		// Firefox на Android не рисует кадр ролика, который не играли.
		try {
			await video.play();
			await painted(video, 1500);
			video.pause();
		} catch {
			return;
		}
		if (!frameIsBlank(video, probeCtx)) return await frameJpeg(video);
		return;
	} catch {
		return;
	} finally {
		video.removeAttribute('src');
		video.load();
		video.remove();
		if (typeof source !== 'string') URL.revokeObjectURL(url);
	}
}

/**
 * Ролик без кадра открыли на этом устройстве: снимаем кадр из уже
 * скачанного файла и кладём в кэш — плитки дальше рисуют его.
 */
export async function keepLocalPoster(
	origin: string,
	videoBlobId: string,
	videoUrl: string
): Promise<string | undefined> {
	try {
		return await getMediaUrl(origin, localPosterId(videoBlobId));
	} catch {
		const jpeg = await videoPosterJpeg(videoUrl);
		return jpeg ? saveLocalPoster(origin, videoBlobId, jpeg) : undefined;
	}
}

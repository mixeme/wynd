/**
 * Запись голоса и видео в приложении (план 46, C14; макеты 4.21–4.27).
 *
 * Чистые помощники отдельно от экрана: какой контейнер умеет браузер, как
 * из уровней звука сделать волну для ленты, как подписать время.
 */

/** Предел голосового — 15 минут, видео — 5 (решено 2026-10-02). */
export const VOICE_MAX_MS = 15 * 60 * 1000;
export const VIDEO_MAX_MS = 5 * 60 * 1000;
/** Уровней волны, что уходят на сервер. */
export const VOICE_PEAKS = 64;

/**
 * Можно ли записывать: нужен защищённый адрес (https или localhost),
 * доступ к микрофону и MediaRecorder. По адресу в сети без https браузер
 * микрофон не даёт — тогда кнопки «Запись» нет (4.21).
 */
export function canRecord(): boolean {
	return (
		typeof window !== 'undefined' &&
		window.isSecureContext &&
		typeof navigator.mediaDevices?.getUserMedia === 'function' &&
		typeof MediaRecorder !== 'undefined'
	);
}

// Голос: сначала Opus в WebM или Ogg — так пишут Chrome, Vivaldi и Firefox.
// MP4 — последним, для Safari: Chromium на Android заявлял его и писал пустой
// файл (Vivaldi, 2026-10-03). Голосовое всё равно перекодируется в AAC.
const AUDIO_TYPES = [
	'audio/webm;codecs=opus',
	'audio/ogg;codecs=opus',
	'audio/webm',
	'audio/mp4;codecs=mp4a.40.2',
	'audio/mp4'
];
// Видео: сначала H.264 и AAC в MP4 — даже если перекодировать не выйдет,
// исходник откроется и на iPhone. VP9 в MP4 перекодировщик не читает.
const VIDEO_TYPES = [
	'video/mp4;codecs=avc1.4D401F,mp4a.40.2',
	'video/mp4;codecs=avc1.42E01E,mp4a.40.2',
	'video/mp4;codecs=avc1,opus',
	'video/mp4;codecs=avc1',
	'video/webm;codecs=h264,opus',
	'video/webm;codecs=vp9,opus',
	'video/webm;codecs=vp8,opus',
	'video/webm',
	'video/mp4'
];

/** Первый контейнер, который браузер пишет; '' — пусть выберет сам. */
export function pickRecorderType(kind: 'audio' | 'video', supports = isTypeSupported): string {
	const list = kind === 'audio' ? AUDIO_TYPES : VIDEO_TYPES;
	return list.find((t) => supports(t)) ?? '';
}

function isTypeSupported(type: string): boolean {
	return typeof MediaRecorder !== 'undefined' && MediaRecorder.isTypeSupported(type);
}

/** Расширение файла по типу записи: m4a / webm / ogg / mp4. */
export function recordingExtension(mime: string): string {
	const base = mime.split(';')[0].trim();
	if (base === 'audio/mp4') return 'm4a';
	if (base === 'video/mp4') return 'mp4';
	if (base.endsWith('/ogg')) return 'ogg';
	return 'webm';
}

/**
 * Волна для ленты: уровни 0–100, снятые во время записи, сжатые до `count`
 * столбиков (максимум в каждом отрезке — тихие паузы не съедают слова).
 */
export function peaksFromLevels(levels: number[], count = VOICE_PEAKS): number[] {
	if (!levels.length) return [];
	if (levels.length <= count) return levels.map((v) => clampLevel(v));
	const out: number[] = [];
	for (let i = 0; i < count; i++) {
		const from = Math.floor((i * levels.length) / count);
		const to = Math.max(from + 1, Math.floor(((i + 1) * levels.length) / count));
		let max = 0;
		for (let j = from; j < to; j++) max = Math.max(max, levels[j]);
		out.push(clampLevel(max));
	}
	return out;
}

function clampLevel(v: number): number {
	return Math.max(0, Math.min(100, Math.round(v)));
}

/** Уровень 0–100 из отсчётов анализатора (0–255, тишина — 128). */
export function levelFromSamples(samples: ArrayLike<number>): number {
	if (!samples.length) return 0;
	let sum = 0;
	for (let i = 0; i < samples.length; i++) {
		const v = (samples[i] - 128) / 128;
		sum += v * v;
	}
	const rms = Math.sqrt(sum / samples.length);
	// Речь — около 0,05–0,3 RMS; корень растягивает тихое, чтобы волна жила.
	return clampLevel(Math.sqrt(rms) * 160);
}

/** «0:48», «12:05» — таймер записи и длительность голосового. */
export function formatDuration(ms: number): string {
	const total = Math.max(0, Math.round(ms / 1000));
	const m = Math.floor(total / 60);
	const s = total % 60;
	return `${m}:${String(s).padStart(2, '0')}`;
}

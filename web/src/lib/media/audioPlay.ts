import { getMediaUrl } from '$lib/media/objectUrl';

// Звук во вложении (4.15–4.20). Одна строка звучит за раз; уход с экрана
// звук не обрывает — внизу любого экрана полоса плеера (4.20), она читает
// отсюда, что играет и откуда.

export interface AudioTrack {
	current: number;
	duration: number;
	started: boolean;
	playing: boolean;
	/** Файл ещё скачивается (4.19): сколько получено из скольких; total 0 — неизвестно. */
	loading?: { received: number; total: number };
}

/** Что играет — для полосы плеера: подпись, обложка, круг и запись. */
export interface AudioMeta {
	title: string;
	coverUrl?: string;
	circleId: string;
	circleName: string;
	/** Цвет круга: линия хода на полосе — цветом того круга, откуда звук. */
	color: string;
	postId: string;
}

// Короткий валидный WAV. play() в том же жесте, что и нажатие, иначе iOS
// не даст запустить уже скачанный файл после await.
const SILENT_WAV =
	'data:audio/wav;base64,UklGRiQAAABXQVZFZm10IBAAAAABAAEARKwAAIhYAQACABAAZGF0YQAAAAA=';

let audio: HTMLAudioElement | null = null;
let token = 0;
let currentKey = '';
let currentOrigin = '';
let currentBlobId = '';
let currentMeta: AudioMeta | undefined;
let loadAbort: AbortController | null = null;
const tracks = new Map<string, AudioTrack>();
const listeners = new Set<() => void>();

export function audioPlayKey(origin: string, blobId: string): string {
	return `${origin}\n${blobId}`;
}

export function audioTrack(key: string): AudioTrack | undefined {
	return tracks.get(key);
}

/** Текущий звук для полосы плеера; нет — полосы нет. */
export function currentAudio():
	| { key: string; origin: string; blobId: string; meta?: AudioMeta; track: AudioTrack }
	| undefined {
	if (!currentKey) return undefined;
	const track = tracks.get(currentKey);
	if (!track?.started) return undefined;
	return { key: currentKey, origin: currentOrigin, blobId: currentBlobId, meta: currentMeta, track };
}

export function subscribeAudio(listener: () => void): () => void {
	listeners.add(listener);
	return () => listeners.delete(listener);
}

function emit() {
	for (const listener of listeners) listener();
}

function finiteDuration(value: number): number {
	return Number.isFinite(value) && value > 0 ? value : 0;
}

function remember(key: string, patch: Partial<AudioTrack>) {
	const prev = tracks.get(key) ?? { current: 0, duration: 0, started: false, playing: false };
	tracks.set(key, { ...prev, ...patch });
	emit();
}

function ensure(): HTMLAudioElement {
	if (audio) return audio;
	const el = new Audio();
	el.preload = 'auto';
	el.addEventListener('timeupdate', () => {
		const key = activeKey();
		if (!key || !realSrc(el)) return;
		remember(key, {
			current: el.currentTime,
			duration: finiteDuration(el.duration),
			started: true,
			playing: !el.paused
		});
	});
	el.addEventListener('durationchange', () => {
		const key = activeKey();
		if (!key || !realSrc(el)) return;
		remember(key, { duration: finiteDuration(el.duration), started: true });
	});
	el.addEventListener('play', () => {
		const key = activeKey();
		if (!key || !realSrc(el)) return;
		remember(key, { playing: true, started: true });
	});
	el.addEventListener('pause', () => {
		const key = activeKey();
		if (!key || !realSrc(el)) return;
		remember(key, { playing: false });
	});
	el.addEventListener('ended', () => {
		const key = activeKey();
		if (!key || !realSrc(el)) return;
		remember(key, { playing: false, current: 0, started: true });
	});
	el.addEventListener('error', () => {
		const key = activeKey();
		if (!key || el.src.startsWith('data:')) return;
		remember(key, { playing: false });
	});
	audio = el;
	return el;
}

function activeKey(): string | undefined {
	return currentKey || undefined;
}

function realSrc(el: HTMLAudioElement): boolean {
	return Boolean(el.src) && !el.src.startsWith('data:');
}

function cancelLoad() {
	loadAbort?.abort();
	loadAbort = null;
}

/** Одна строка звучит. Новая ставит предыдущую на паузу, место не сбрасывает. */
export async function toggleAudio(origin: string, blobId: string, meta?: AudioMeta): Promise<void> {
	const key = audioPlayKey(origin, blobId);
	const el = ensure();
	if (meta && currentKey === key) currentMeta = meta;

	if (currentKey === key && tracks.get(key)?.started && realSrc(el)) {
		if (el.paused) {
			if (el.ended) el.currentTime = 0;
			try {
				await el.play();
			} catch {
				remember(key, { playing: false });
			}
		} else {
			el.pause();
		}
		return;
	}

	// Второе нажатие, пока файл скачивается, — отмена (4.19).
	if (currentKey === key && !realSrc(el)) {
		token += 1;
		cancelLoad();
		el.pause();
		remember(key, { playing: false, started: false, current: 0, loading: undefined });
		return;
	}

	const my = ++token;
	cancelLoad();
	if (currentKey && currentKey !== key) {
		const prev = currentKey;
		el.pause();
		remember(prev, {
			playing: false,
			loading: undefined,
			current: realSrc(el) ? el.currentTime : (tracks.get(prev)?.current ?? 0),
			duration: finiteDuration(el.duration) || tracks.get(prev)?.duration || 0
		});
	}

	currentKey = key;
	currentOrigin = origin;
	currentBlobId = blobId;
	currentMeta = meta;
	el.dataset.key = key;
	el.src = SILENT_WAV;
	void el.play().catch(() => {});
	remember(key, {
		started: true,
		playing: false,
		current: tracks.get(key)?.current ?? 0,
		loading: { received: 0, total: 0 }
	});

	const ctrl = new AbortController();
	loadAbort = ctrl;
	let url: string;
	try {
		url = await getMediaUrl(origin, blobId, {
			signal: ctrl.signal,
			onProgress: (received, total) => {
				if (my === token) remember(key, { loading: { received, total } });
			}
		});
	} catch {
		if (my !== token) return;
		remember(key, { playing: false, started: false, loading: undefined });
		return;
	} finally {
		if (loadAbort === ctrl) loadAbort = null;
	}
	if (my !== token || currentKey !== key) return;
	remember(key, { loading: undefined });
	const resumeAt = tracks.get(key)?.current ?? 0;
	el.src = url;
	if (resumeAt > 0) el.currentTime = resumeAt;
	try {
		await el.play();
	} catch {
		if (my !== token) return;
		remember(key, { playing: false });
	}
}

/** Крестик полосы (4.20): звук смолкает, полоса уходит. */
export function stopAudio() {
	if (!currentKey) return;
	const key = currentKey;
	token += 1;
	cancelLoad();
	audio?.pause();
	audio?.removeAttribute('src');
	if (audio) audio.dataset.key = '';
	currentKey = '';
	currentOrigin = '';
	currentBlobId = '';
	currentMeta = undefined;
	remember(key, { playing: false, started: false, current: 0, loading: undefined });
}

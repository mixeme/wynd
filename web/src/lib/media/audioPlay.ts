import { getMediaUrl } from '$lib/media/objectUrl';

export interface AudioTrack {
	current: number;
	duration: number;
	started: boolean;
	playing: boolean;
}

// Короткий валидный WAV. play() в том же жесте, что и нажатие, иначе iOS
// не даст запустить уже скачанный файл после await.
const SILENT_WAV =
	'data:audio/wav;base64,UklGRiQAAABXQVZFZm10IBAAAAABAAEARKwAAIhYAQACABAAZGF0YQAAAAA=';

let audio: HTMLAudioElement | null = null;
let token = 0;
let currentKey = '';
const tracks = new Map<string, AudioTrack>();
const listeners = new Set<() => void>();

export function audioPlayKey(origin: string, blobId: string): string {
	return `${origin}\n${blobId}`;
}

export function audioTrack(key: string): AudioTrack | undefined {
	return tracks.get(key);
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

/** Одна строка звучит. Новая ставит предыдущую на паузу, место не сбрасывает. */
export async function toggleAudio(origin: string, blobId: string): Promise<void> {
	const key = audioPlayKey(origin, blobId);
	const el = ensure();

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

	if (currentKey === key && !realSrc(el)) {
		token += 1;
		el.pause();
		remember(key, { playing: false, started: false, current: 0 });
		return;
	}

	const my = ++token;
	if (currentKey && currentKey !== key) {
		const prev = currentKey;
		el.pause();
		remember(prev, {
			playing: false,
			current: realSrc(el) ? el.currentTime : (tracks.get(prev)?.current ?? 0),
			duration: finiteDuration(el.duration) || tracks.get(prev)?.duration || 0
		});
	}

	currentKey = key;
	el.dataset.key = key;
	el.src = SILENT_WAV;
	void el.play().catch(() => {});
	remember(key, { started: true, playing: false, current: tracks.get(key)?.current ?? 0 });

	let url: string;
	try {
		url = await getMediaUrl(origin, blobId);
	} catch {
		if (my !== token) return;
		remember(key, { playing: false, started: false });
		return;
	}
	if (my !== token || currentKey !== key) return;
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

export function stopAudioIf(key: string) {
	if (!audio || currentKey !== key) return;
	token += 1;
	audio.pause();
	audio.removeAttribute('src');
	audio.dataset.key = '';
	currentKey = '';
	const prev = tracks.get(key);
	if (prev) remember(key, { playing: false });
}

import { holdWakeLock } from './wake-lock';
import {
	VOICE_MAX_MS,
	VOICE_PEAKS,
	levelFromSamples,
	levelsFromPcm,
	peaksFromLevels,
	pickRecorderType,
	recordingExtension
} from './record';

export type VoicePhase = 'idle' | 'recording' | 'review';

/** Готовое голосовое: файл, длительность и волна для ленты. */
export interface VoiceTake {
	file: File;
	durationMs: number;
	peaks: number[];
}

/**
 * Диктофон полосы ввода (4.23, 4.24): микрофон, таймер, предел 15 минут.
 * Экран не гаснет, пока идёт запись. Всё, что взято у браузера (поток,
 * адрес файла), отдаётся в `dispose`.
 *
 * Живая волна — с копии микрофона: запись получает звук напрямую, анализатор
 * Web Audio слушает свою дорожку (`stream.clone()`). До 0.22.5 анализатор
 * сидел на той же дорожке, что и запись, и на телефоне она выходила рваной,
 * неполной или пустой (realme 8, Firefox и Vivaldi, 2026-10-07); без него
 * запись чистая. На отдельной странице диагностики чистыми вышли все способы,
 * включая прежний, так что точная причина не установлена — если запись снова
 * испортится, убирать `#meter` первым. Волна для ленты от живой не зависит:
 * считается из готовой записи после «Стоп».
 */
export class VoiceRecorder {
	phase = $state<VoicePhase>('idle');
	elapsedMs = $state(0);
	/** Последние уровни — живая волна во время записи; без анализатора ровная. */
	recent = $state<number[]>([]);
	/** Адрес записанного — прослушать перед отправкой. */
	url = $state('');
	error = $state('');

	#stream: MediaStream | null = null;
	#recorder: MediaRecorder | null = null;
	#chunks: Blob[] = [];
	#meterStream: MediaStream | null = null;
	#ctx: AudioContext | null = null;
	#analyser: AnalyserNode | null = null;
	#tick: ReturnType<typeof setInterval> | null = null;
	#startedAt = 0;
	#releaseWake: (() => void) | null = null;
	#take: VoiceTake | null = null;

	get take(): VoiceTake | null {
		return this.#take;
	}

	get peaks(): number[] {
		return this.#take?.peaks ?? [];
	}

	async start(): Promise<void> {
		if (this.phase !== 'idle') return;
		this.error = '';
		try {
			this.#stream = await navigator.mediaDevices.getUserMedia({ audio: true });
		} catch {
			this.error = 'Нет доступа к микрофону';
			return;
		}
		const type = pickRecorderType('audio');
		this.#recorder = new MediaRecorder(this.#stream, type ? { mimeType: type } : undefined);
		this.#chunks = [];
		this.#recorder.ondataavailable = (e) => {
			if (e.data.size) this.#chunks.push(e.data);
		};
		this.recent = [];
		this.#meter(this.#stream);
		// Без нарезки по секунде: Firefox на Android писал рваный звук на стыках.
		this.#recorder.start();
		this.#startedAt = performance.now();
		this.elapsedMs = 0;
		this.phase = 'recording';
		this.#releaseWake = holdWakeLock();
		this.#tick = setInterval(() => this.#onTick(), 100);
	}

	/** Стоп — к проверке (4.24). */
	async stop(): Promise<void> {
		const recorder = this.#recorder;
		if (this.phase !== 'recording' || !recorder) return;
		const durationMs = performance.now() - this.#startedAt;
		const done = new Promise<void>((resolve) => {
			recorder.onstop = () => resolve();
		});
		recorder.stop();
		await done;
		const type = recorder.mimeType || pickRecorderType('audio') || 'audio/webm';
		this.#releaseInputs();
		const blob = new Blob(this.#chunks, { type });
		this.#chunks = [];
		if (!blob.size) {
			this.error = 'Запись пустая';
			this.phase = 'idle';
			return;
		}
		const file = new File([blob], `Голосовое.${recordingExtension(type)}`, { type: blob.type });
		this.#take = { file, durationMs, peaks: await peaksOf(blob) };
		this.url = URL.createObjectURL(blob);
		this.elapsedMs = durationMs;
		this.phase = 'review';
	}

	/** «Отмена» во время записи и корзина на проверке — выбросить. */
	discard(): void {
		if (this.#recorder && this.#recorder.state !== 'inactive') {
			this.#recorder.onstop = null;
			this.#recorder.stop();
		}
		this.#releaseInputs();
		this.#chunks = [];
		this.#take = null;
		if (this.url) URL.revokeObjectURL(this.url);
		this.url = '';
		this.recent = [];
		this.elapsedMs = 0;
		this.phase = 'idle';
	}

	dispose(): void {
		this.discard();
	}

	#meter(stream: MediaStream) {
		try {
			const Ctx =
				globalThis.AudioContext ??
				(globalThis as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
			if (!Ctx) return;
			this.#meterStream = stream.clone();
			this.#ctx = new Ctx();
			// После запроса микрофона контекст может родиться приостановленным.
			void this.#ctx.resume().catch(() => {});
			this.#analyser = this.#ctx.createAnalyser();
			this.#analyser.fftSize = 512;
			this.#ctx.createMediaStreamSource(this.#meterStream).connect(this.#analyser);
		} catch {
			// Без уровня запись всё равно идёт — волна будет ровной.
			this.#analyser = null;
		}
	}

	#onTick() {
		this.elapsedMs = performance.now() - this.#startedAt;
		let level = 0;
		if (this.#analyser) {
			const buf = new Uint8Array(this.#analyser.fftSize);
			this.#analyser.getByteTimeDomainData(buf);
			level = levelFromSamples(buf);
		}
		this.recent = [...this.recent.slice(-17), level];
		if (this.elapsedMs >= VOICE_MAX_MS) void this.stop();
	}

	#releaseInputs() {
		if (this.#tick) clearInterval(this.#tick);
		this.#tick = null;
		this.#stream?.getTracks().forEach((t) => t.stop());
		this.#stream = null;
		this.#meterStream?.getTracks().forEach((t) => t.stop());
		this.#meterStream = null;
		void this.#ctx?.close().catch(() => {});
		this.#ctx = null;
		this.#analyser = null;
		this.#recorder = null;
		this.#releaseWake?.();
		this.#releaseWake = null;
	}
}

/**
 * Волна для ленты из готовой записи. Декодируем в 8 кГц и моно: четверть
 * часа — около 30 МБ отсчётов, а не сотни. Не разобралось — ровная волна.
 */
async function peaksOf(blob: Blob): Promise<number[]> {
	try {
		const Offline =
			globalThis.OfflineAudioContext ??
			(globalThis as { webkitOfflineAudioContext?: typeof OfflineAudioContext })
				.webkitOfflineAudioContext;
		if (!Offline) throw new Error('no_decoder');
		const rate = 8000;
		const decoded = await new Offline(1, 1, rate).decodeAudioData(await blob.arrayBuffer());
		const peaks = peaksFromLevels(levelsFromPcm(decoded.getChannelData(0), decoded.sampleRate));
		if (peaks.length) return peaks;
	} catch {
		// ниже — ровная волна
	}
	return new Array<number>(VOICE_PEAKS).fill(0);
}

import { holdWakeLock } from './wake-lock';
import {
	VOICE_MAX_MS,
	levelFromSamples,
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
 * Диктофон полосы ввода (4.23, 4.24): микрофон, таймер, уровень звука,
 * предел 15 минут. Экран не гаснет, пока идёт запись. Всё, что взято у
 * браузера (поток, контекст звука, адрес файла), отдаётся в `dispose`.
 */
export class VoiceRecorder {
	phase = $state<VoicePhase>('idle');
	elapsedMs = $state(0);
	/** Последние уровни — живая волна во время записи. */
	recent = $state<number[]>([]);
	/** Адрес записанного — прослушать перед отправкой. */
	url = $state('');
	error = $state('');

	#stream: MediaStream | null = null;
	#recorder: MediaRecorder | null = null;
	#chunks: Blob[] = [];
	#ctx: AudioContext | null = null;
	#analyser: AnalyserNode | null = null;
	#tick: ReturnType<typeof setInterval> | null = null;
	#startedAt = 0;
	#levels: number[] = [];
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
		this.#levels = [];
		this.recent = [];
		this.#meter(this.#stream);
		this.#recorder.start(1000);
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
		this.#take = { file, durationMs, peaks: peaksFromLevels(this.#levels) };
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
			this.#ctx = new Ctx();
			const source = this.#ctx.createMediaStreamSource(stream);
			this.#analyser = this.#ctx.createAnalyser();
			this.#analyser.fftSize = 512;
			source.connect(this.#analyser);
		} catch {
			// Без уровня запись всё равно идёт — волна будет ровной.
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
		this.#levels.push(level);
		this.recent = [...this.recent.slice(-17), level];
		if (this.elapsedMs >= VOICE_MAX_MS) void this.stop();
	}

	#releaseInputs() {
		if (this.#tick) clearInterval(this.#tick);
		this.#tick = null;
		this.#stream?.getTracks().forEach((t) => t.stop());
		this.#stream = null;
		void this.#ctx?.close().catch(() => {});
		this.#ctx = null;
		this.#analyser = null;
		this.#recorder = null;
		this.#releaseWake?.();
		this.#releaseWake = null;
	}
}

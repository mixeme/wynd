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
 * Диктофон полосы ввода (4.12, 4.13): микрофон, таймер, предел 15 минут.
 * Экран не гаснет, пока идёт запись. Всё, что взято у браузера (поток,
 * адрес файла), отдаётся в `dispose`.
 *
 * Живая волна — с копии микрофона: запись получает звук напрямую, анализатор
 * Web Audio слушает свою дорожку (`stream.clone()`). Волна для ленты от живой
 * не зависит: считается из готовой записи после «Стоп».
 *
 * Рваная, неполная или пустая запись на realme 8 (2026-10-07), по наблюдению
 * с телефона, шла через микрофон подключённых по Bluetooth часов: браузер
 * берёт устройство связи, какое дала система. Анализатор тут ни при чём — на
 * странице диагностики чистыми вышли все способы записи, включая прежний.
 */
export class VoiceRecorder {
	phase = $state<VoicePhase>('idle');
	elapsedMs = $state(0);
	/** Последние уровни — живая волна во время записи; без анализатора ровная. */
	recent = $state<number[]>([]);
	/** Адрес записанного — прослушать перед отправкой. */
	url = $state('');
	error = $state('');

	#attempt: symbol | null = null;
	#stream: MediaStream | null = null;
	#recorder: MediaRecorder | null = null;
	#meterStream: MediaStream | null = null;
	#ctx: AudioContext | null = null;
	#analyser: AnalyserNode | null = null;
	#tick: ReturnType<typeof setInterval> | null = null;
	#startedAt = 0;
	#stoppedAt: number | null = null;
	#releaseWake: (() => void) | null = null;
	#take: VoiceTake | null = null;

	get take(): VoiceTake | null {
		return this.#take;
	}

	get peaks(): number[] {
		return this.#take?.peaks ?? [];
	}

	async start(): Promise<void> {
		if (this.phase !== 'idle' || this.#attempt) return;
		const attempt = (this.#attempt = Symbol());
		this.error = '';
		let stream: MediaStream;
		try {
			stream = await navigator.mediaDevices.getUserMedia({ audio: true });
		} catch {
			if (this.#attempt === attempt) {
				this.#attempt = null;
				this.error = 'Нет доступа к микрофону';
			}
			return;
		}
		if (this.#attempt !== attempt) {
			stream.getTracks().forEach((track) => track.stop());
			return;
		}
		this.#stream = stream;
		try {
			const type = pickRecorderType('audio');
			const recorder = (this.#recorder = new MediaRecorder(
				stream,
				type ? { mimeType: type } : undefined
			));
			const chunks: Blob[] = [];
			recorder.ondataavailable = ({ data }) => {
				if (data.size) chunks.push(data);
			};
			// stop приходит и при завершении потока без нажатия «Стоп».
			recorder.onstop = () => void this.#finish(attempt, recorder, chunks);
			this.recent = [];
			this.#meter(stream);
			// Без нарезки по секунде: Firefox на Android писал рваный звук на стыках.
			recorder.start();
		} catch {
			this.discard();
			this.error = 'Не удалось начать запись';
			return;
		}
		this.#startedAt = performance.now();
		this.#stoppedAt = null;
		this.elapsedMs = 0;
		this.phase = 'recording';
		this.#releaseWake = holdWakeLock();
		this.#tick = setInterval(() => this.#onTick(), 100);
	}

	/** Стоп — к проверке (4.13). */
	stop(): void {
		const recorder = this.#recorder;
		if (!recorder || recorder.state === 'inactive' || this.#stoppedAt !== null) return;
		this.#stoppedAt = performance.now();
		recorder.stop();
	}

	async #finish(attempt: symbol, recorder: MediaRecorder, chunks: Blob[]): Promise<void> {
		if (this.#attempt !== attempt || this.#recorder !== recorder) return;
		const elapsed = (this.#stoppedAt ?? performance.now()) - this.#startedAt;
		const type = recorder.mimeType || pickRecorderType('audio') || 'audio/webm';
		this.#releaseInputs();
		const blob = new Blob(chunks, { type });
		if (!blob.size) {
			this.#attempt = null;
			this.error = 'Запись пустая';
			this.phase = 'idle';
			return;
		}
		const { peaks, duration } = await voiceDetails(blob);
		if (this.#attempt !== attempt) return;
		this.#attempt = null;
		const durationMs = duration ?? elapsed;
		const file = new File([blob], `Голосовое.${recordingExtension(type)}`, { type: blob.type });
		this.#take = { file, durationMs, peaks };
		this.url = URL.createObjectURL(blob);
		this.elapsedMs = durationMs;
		this.phase = 'review';
	}

	/** «Отмена» во время записи и корзина на проверке — выбросить. */
	discard(): void {
		this.#attempt = null;
		if (this.#recorder) {
			this.#recorder.onstop = null;
			this.#recorder.ondataavailable = null;
			if (this.#recorder.state !== 'inactive') this.#recorder.stop();
		}
		this.#releaseInputs();
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
		if (this.elapsedMs >= VOICE_MAX_MS) this.stop();
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
 * Волна и длительность из готовой записи. Декодируем в 8 кГц и моно:
 * четверть часа — около 30 МБ отсчётов. Если декодера нет, оставляем ровную волну.
 */
async function voiceDetails(blob: Blob): Promise<{ peaks: number[]; duration?: number }> {
	try {
		const Offline =
			globalThis.OfflineAudioContext ??
			(globalThis as { webkitOfflineAudioContext?: typeof OfflineAudioContext })
				.webkitOfflineAudioContext;
		if (!Offline) throw new Error('no_decoder');
		const rate = 8000;
		const decoded = await new Offline(1, 1, rate).decodeAudioData(await blob.arrayBuffer());
		const peaks = peaksFromLevels(levelsFromPcm(decoded.getChannelData(0), decoded.sampleRate));
		if (peaks.length) return { peaks, duration: decoded.duration * 1000 };
	} catch {
		// ниже — ровная волна
	}
	return { peaks: new Array<number>(VOICE_PEAKS).fill(0) };
}

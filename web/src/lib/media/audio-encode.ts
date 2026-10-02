import type { CompressionSettings } from '$lib/journal/types';
import {
	audioFitsSettings,
	DEFAULT_AUDIO_BITRATE_KBPS,
	fileToQueueBuffer,
	type CompressedMedia
} from './compress';

/**
 * Сжатие звука перед загрузкой (план 46, A6): WAV или FLAC с телефона весит
 * десятки мегабайт. Перекодируем в AAC (или Opus, если AAC браузер не умеет)
 * в MP4 с битрейтом из настроек инстанса. Уже сжатый звук не выше порога
 * уходит как есть; результат не меньше оригинала — тоже оригинал. Теги и
 * обложку экран читает из исходного файла до сжатия.
 *
 * Без WebCodecs (Firefox на Android) у браузера нет ни кодировщика, ни
 * декодера: FLAC уходил исходником. Тогда AAC кодирует WASM-сборка
 * (`@mediabunny/aac-encoder`), а читает звук Web Audio (`decodeAudioData`),
 * которая есть везде.
 */
export async function encodeAudio(
	file: File,
	settings?: CompressionSettings,
	onProgress?: (progress: number) => void,
	/** Голосовое (C14): свой битрейт и перекодировать всегда — запись
	 *  браузера в WebM или Ogg может не сыграть на iPhone. */
	opts?: { bitrateKbps?: number; force?: boolean }
): Promise<CompressedMedia> {
	const bitrateKbps = opts?.bitrateKbps || settings?.audio_bitrate_kbps || DEFAULT_AUDIO_BITRATE_KBPS;
	const bitrate = bitrateKbps * 1000;

	const mb = await import('mediabunny');
	const { ALL_FORMATS, BlobSource, Input, Mp4OutputFormat, Quality, getFirstEncodableAudioCodec } = mb;

	const input = new Input({ source: new BlobSource(file), formats: ALL_FORMATS });
	try {
		const track = await input.getPrimaryAudioTrack();
		if (!track) throw new Error('no_audio_track');
		const duration = (await input.getDurationFromMetadata()) ?? 0;
		if (!opts?.force && audioFitsSettings(track.codec, duration, file.size, bitrateKbps)) {
			return fileToQueueBuffer(file);
		}

		const format = new Mp4OutputFormat({ fastStart: 'in-memory' });
		const quality = new Quality({ bitrate, bitrateMode: 'variable' });
		await ensureAacEncoder();
		const codec = await getFirstEncodableAudioCodec(
			format.getSupportedAudioCodecs().filter((c) => c === 'aac' || c === 'opus'),
			{ numberOfChannels: track.numberOfChannels, sampleRate: track.sampleRate, quality }
		);
		if (!codec) throw new Error('no_encoder');

		const buffer = (await track.canDecode())
			? await convertWithMediabunny(input, codec, quality, onProgress)
			: await encodeViaWebAudio(file, codec, quality, onProgress);
		if (!buffer || buffer.byteLength === 0) throw new Error('empty_output');
		if (!opts?.force && buffer.byteLength >= file.size) return fileToQueueBuffer(file);

		const base = file.name.replace(/\.[^.]+$/, '') || 'audio';
		return { data: buffer, type: 'audio/mp4', name: `${base}.m4a`, size: buffer.byteLength };
	} finally {
		input.dispose();
	}
}

let aacRegistered: Promise<void> | undefined;

/** Нет своего AAC в браузере — регистрируем WASM-кодировщик (один раз). */
export function ensureAacEncoder(): Promise<void> {
	aacRegistered ??= (async () => {
		const { canEncodeAudio } = await import('mediabunny');
		if (await canEncodeAudio('aac')) return;
		const { registerAacEncoder } = await import('@mediabunny/aac-encoder');
		registerAacEncoder();
	})();
	return aacRegistered;
}

type Mb = typeof import('mediabunny');

async function convertWithMediabunny(
	input: InstanceType<Mb['Input']>,
	codec: import('mediabunny').AudioCodec,
	quality: InstanceType<Mb['Quality']>,
	onProgress?: (progress: number) => void
): Promise<ArrayBuffer | null> {
	const { BufferTarget, Conversion, Mp4OutputFormat, Output } = await import('mediabunny');
	const target = new BufferTarget();
	const output = new Output({ format: new Mp4OutputFormat({ fastStart: 'in-memory' }), target });
	const conversion = await Conversion.init({
		input,
		output,
		tracks: 'primary',
		video: { discard: true },
		audio: { codec, quality, forceTranscode: true }
	});
	if (!conversion.isValid) throw new Error('conversion_invalid');
	if (!conversion.utilizedTracks.some((t) => t.isAudioTrack())) throw new Error('audio_discarded');
	conversion.onProgress = (progress) => onProgress?.(progress);
	await conversion.execute();
	return target.buffer;
}

// Кусок звука на один вызов кодировщика: полный AudioBuffer второй копией
// не держим.
const CHUNK_SEC = 10;

async function encodeViaWebAudio(
	file: File,
	codec: import('mediabunny').AudioCodec,
	quality: InstanceType<Mb['Quality']>,
	onProgress?: (progress: number) => void
): Promise<ArrayBuffer | null> {
	const { AudioBufferSource, BufferTarget, Mp4OutputFormat, Output } = await import('mediabunny');
	const Ctx = globalThis.AudioContext ?? (globalThis as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
	if (!Ctx) throw new Error('no_decoder');
	const ctx = new Ctx();
	let decoded: AudioBuffer;
	try {
		decoded = await ctx.decodeAudioData(await file.arrayBuffer());
	} catch {
		throw new Error('no_decoder');
	} finally {
		void ctx.close();
	}

	const target = new BufferTarget();
	const output = new Output({ format: new Mp4OutputFormat({ fastStart: 'in-memory' }), target });
	const source = new AudioBufferSource({ codec, quality });
	output.addAudioTrack(source);
	await output.start();

	const { numberOfChannels, sampleRate, length } = decoded;
	const step = CHUNK_SEC * sampleRate;
	for (let start = 0; start < length; start += step) {
		const frames = Math.min(step, length - start);
		const chunk = new AudioBuffer({ length: frames, numberOfChannels, sampleRate });
		for (let ch = 0; ch < numberOfChannels; ch++) {
			chunk.copyToChannel(decoded.getChannelData(ch).subarray(start, start + frames), ch);
		}
		await source.add(chunk);
		onProgress?.((start + frames) / length);
	}
	await output.finalize();
	return target.buffer;
}

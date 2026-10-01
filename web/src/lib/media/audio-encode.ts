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
 */
export async function encodeAudio(
	file: File,
	settings?: CompressionSettings,
	onProgress?: (progress: number) => void
): Promise<CompressedMedia> {
	const bitrateKbps = settings?.audio_bitrate_kbps || DEFAULT_AUDIO_BITRATE_KBPS;
	const bitrate = bitrateKbps * 1000;

	const {
		ALL_FORMATS,
		BlobSource,
		BufferTarget,
		Conversion,
		Input,
		Mp4OutputFormat,
		Output,
		Quality,
		getFirstEncodableAudioCodec
	} = await import('mediabunny');

	const input = new Input({ source: new BlobSource(file), formats: ALL_FORMATS });
	try {
		const track = await input.getPrimaryAudioTrack();
		if (!track) throw new Error('no_audio_track');
		const duration = (await input.getDurationFromMetadata()) ?? 0;
		if (audioFitsSettings(track.codec, duration, file.size, bitrateKbps)) {
			return fileToQueueBuffer(file);
		}

		const format = new Mp4OutputFormat({ fastStart: 'in-memory' });
		const quality = new Quality({ bitrate, bitrateMode: 'variable' });
		const codec = await getFirstEncodableAudioCodec(
			format.getSupportedAudioCodecs().filter((c) => c === 'aac' || c === 'opus'),
			{ numberOfChannels: track.numberOfChannels, sampleRate: track.sampleRate, quality }
		);
		if (!codec) throw new Error('no_encoder');

		const target = new BufferTarget();
		const output = new Output({ format, target });
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

		const buffer = target.buffer;
		if (!buffer || buffer.byteLength === 0) throw new Error('empty_output');
		if (buffer.byteLength >= file.size) return fileToQueueBuffer(file);

		const base = file.name.replace(/\.[^.]+$/, '') || 'audio';
		return { data: buffer, type: 'audio/mp4', name: `${base}.m4a`, size: buffer.byteLength };
	} finally {
		input.dispose();
	}
}

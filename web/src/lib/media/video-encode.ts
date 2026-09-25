import type { CompressionSettings } from '$lib/journal/types';
import {
	DEFAULT_VIDEO_BITRATE_KBPS,
	DEFAULT_VIDEO_MAX_P,
	fileToQueueBuffer,
	targetVideoSize,
	videoFitsSettings,
	type CompressedMedia
} from './compress';

export async function encodeVideo(
	file: File,
	settings?: CompressionSettings,
	onProgress?: (progress: number) => void
): Promise<CompressedMedia> {
	const maxP = settings?.video_max_height || DEFAULT_VIDEO_MAX_P;
	const bitrateKbps = settings?.video_bitrate_kbps || DEFAULT_VIDEO_BITRATE_KBPS;
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
		getFirstEncodableVideoCodec
	} = await import('mediabunny');

	const input = new Input({
		source: new BlobSource(file),
		formats: ALL_FORMATS
	});

	try {
		const track = await input.getPrimaryVideoTrack();
		if (!track) throw new Error('no_video_track');

		const width = await track.getDisplayWidth();
		const height = await track.getDisplayHeight();
		const duration = (await input.getDurationFromMetadata()) ?? 0;
		if (videoFitsSettings(width, height, duration, file.size, maxP, bitrateKbps)) {
			return fileToQueueBuffer(file);
		}

		const target = targetVideoSize(width, height, maxP);
		const format = new Mp4OutputFormat({ fastStart: 'in-memory' });
		const codec = await getFirstEncodableVideoCodec(format.getSupportedVideoCodecs(), {
			width: target.width,
			height: target.height,
			quality: new Quality({ bitrate, bitrateMode: 'variable' })
		});
		if (!codec) throw new Error('no_encoder');

		const bufferTarget = new BufferTarget();
		const output = new Output({ format, target: bufferTarget });
		const quality = new Quality({ bitrate, bitrateMode: 'variable' });
		const conversion = await Conversion.init({
			input,
			output,
			tracks: 'primary',
			video: {
				width: target.width,
				height: target.height,
				fit: 'contain',
				codec,
				quality,
				hardwareAcceleration: 'no-preference'
			}
		});
		if (!conversion.isValid) throw new Error('conversion_invalid');
		if (!conversion.utilizedTracks.some((t) => t.isVideoTrack())) throw new Error('video_discarded');
		const audioDropped = conversion.discardedTracks.some(
			(d) =>
				d.track.isAudioTrack() &&
				(d.reason === 'no_encodable_target_codec' ||
					d.reason === 'undecodable_source_codec' ||
					d.reason === 'unknown_source_codec')
		);
		if (audioDropped) throw new Error('audio_discarded');

		conversion.onProgress = (progress) => onProgress?.(progress);
		await conversion.execute();

		const buffer = bufferTarget.buffer;
		if (!buffer || buffer.byteLength === 0) throw new Error('empty_output');
		if (buffer.byteLength >= file.size) return fileToQueueBuffer(file);

		const base = file.name.replace(/\.[^.]+$/, '') || 'video';
		return {
			data: buffer,
			type: 'video/mp4',
			name: `${base}.mp4`,
			size: buffer.byteLength
		};
	} finally {
		input.dispose();
	}
}

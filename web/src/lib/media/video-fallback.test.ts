import { describe, expect, it } from 'vitest';
import { videoFallbackReason } from './compress';

// Видео, которое не сжалось, должно объяснить почему: иначе 4K «как есть»
// упирался в потолок вложения, и на экране было только «больше 100 МБ».
describe('videoFallbackReason', () => {
	it('names known encoder failures in words', () => {
		expect(videoFallbackReason(new Error('no_encoder'))).toBe('браузер не умеет кодировать видео');
		expect(videoFallbackReason(new Error('audio_discarded'))).toContain('звук');
		expect(videoFallbackReason(new Error('video_discarded'))).toContain('формат');
	});

	it('falls back to a generic reason, never an empty string', () => {
		expect(videoFallbackReason(new Error('boom'))).not.toBe('');
		expect(videoFallbackReason('x')).not.toBe('');
	});
});

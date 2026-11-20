import { Blob, File } from 'node:buffer';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { VoiceRecorder } from './voiceRecorder.svelte';

class TestRecorder {
	static latest: TestRecorder;
	static isTypeSupported = () => true;
	state: RecordingState = 'inactive';
	mimeType = 'audio/webm;codecs=opus';
	ondataavailable: ((event: { data: Blob }) => void) | null = null;
	onstop: (() => void) | null = null;
	stop = vi.fn(() => this.end());

	constructor() {
		TestRecorder.latest = this;
	}
	start() {
		this.state = 'recording';
	}
	end(data = new Blob(['audio'])) {
		this.state = 'inactive';
		queueMicrotask(() => {
			this.ondataavailable?.({ data });
			this.onstop?.();
		});
	}
}

const microphone = { getTracks: () => [{ stop: vi.fn() }] };
const getUserMedia = vi.fn(async () => microphone);
const decodedAudio = {
	duration: 1,
	sampleRate: 8000,
	getChannelData: () => new Float32Array(8000)
};
const decode = vi.fn(async () => decodedAudio);
let recorder: VoiceRecorder;

beforeEach(() => {
	vi.useFakeTimers();
	vi.stubGlobal('Blob', Blob);
	vi.stubGlobal('File', File);
	vi.stubGlobal('MediaRecorder', TestRecorder);
	vi.stubGlobal('navigator', { mediaDevices: { getUserMedia } });
	vi.stubGlobal(
		'OfflineAudioContext',
		class {
			decodeAudioData = decode;
		}
	);
	vi.stubGlobal('URL', { createObjectURL: vi.fn(() => 'blob:take'), revokeObjectURL: vi.fn() });
	recorder = new VoiceRecorder();
});

afterEach(() => {
	recorder.dispose();
	vi.useRealTimers();
	vi.unstubAllGlobals();
	vi.clearAllMocks();
});

it('для короткого файла показывает длительность файла и завершает запись по первому нажатию', async () => {
	await recorder.start();
	vi.advanceTimersByTime(6000);
	expect(recorder.elapsedMs).toBe(6000);
	recorder.stop();
	recorder.stop();
	await vi.waitFor(() => expect(recorder.phase).toBe('review'));
	expect(TestRecorder.latest.stop).toHaveBeenCalledTimes(1);
	expect(recorder.elapsedMs).toBe(1000);
	expect(recorder.take?.durationMs).toBe(1000);
	expect(recorder.take?.peaks).toHaveLength(10);
});

it('при завершении потока открывает прослушивание без повторного вызова stop', async () => {
	await recorder.start();
	TestRecorder.latest.end();
	recorder.stop();
	await vi.waitFor(() => expect(recorder.phase).toBe('review'));
	expect(TestRecorder.latest.stop).not.toHaveBeenCalled();
	expect(recorder.url).toBe('blob:take');
});

it('пустая прерванная запись возвращает строку в состояние покоя', async () => {
	await recorder.start();
	TestRecorder.latest.end(new Blob([]));
	await vi.waitFor(() => expect(recorder.phase).toBe('idle'));
	expect(recorder.error).toBe('Запись пустая');
	expect(recorder.take).toBeNull();
});

it('использует таймер, если браузер не смог декодировать файл', async () => {
	decode.mockRejectedValueOnce(new Error('Unsupported codec'));
	await recorder.start();
	vi.advanceTimersByTime(4300);
	recorder.stop();
	await vi.waitFor(() => expect(recorder.phase).toBe('review'));
	expect(recorder.take?.durationMs).toBe(4300);
	expect(recorder.take?.peaks).toEqual(Array(64).fill(0));
});

it('отмена во время декодирования не возвращает удалённый файл', async () => {
	const pending = Promise.withResolvers<typeof decodedAudio>();
	decode.mockReturnValueOnce(pending.promise);
	await recorder.start();
	recorder.stop();
	await vi.waitFor(() => expect(decode).toHaveBeenCalledTimes(1));
	recorder.discard();
	pending.resolve(decodedAudio);
	await vi.advanceTimersByTimeAsync(0);
	expect(recorder.phase).toBe('idle');
	expect(recorder.take).toBeNull();
	expect(recorder.url).toBe('');
});

it('отмена запроса доступа освобождает поздно полученный микрофон', async () => {
	const pending = Promise.withResolvers<typeof microphone>();
	const stop = vi.fn();
	getUserMedia.mockReturnValueOnce(pending.promise);
	const starting = recorder.start();
	await recorder.start();
	expect(getUserMedia).toHaveBeenCalledTimes(1);
	recorder.dispose();
	pending.resolve({ getTracks: () => [{ stop }] });
	await starting;
	expect(stop).toHaveBeenCalledTimes(1);
	expect(recorder.phase).toBe('idle');
});

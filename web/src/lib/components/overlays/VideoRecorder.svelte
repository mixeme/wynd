<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import { modal } from '$lib/a11y/modal';
	import { formatBytes } from '$lib/format/bytes';
	import type { CompressionSettings } from '$lib/journal/types';
	import {
		DEFAULT_VIDEO_BITRATE_KBPS,
		DEFAULT_VIDEO_MAX_P,
		targetVideoSize
	} from '$lib/media/compress';
	import { holdWakeLock } from '$lib/media/wake-lock';
	import {
		VIDEO_MAX_MS,
		formatDuration,
		cameraAsks,
		cameraFailure,
		pickRecorderType,
		recordingExtension
	} from '$lib/media/record';
	import { liveFrameJpeg } from '$lib/media/videoPoster';

	// Запись видео (4.25) и просмотр перед отправкой (4.26). Камера во весь
	// экран, задняя по умолчанию; касание большой кнопки — старт и стоп;
	// предел — 5 минут. Видео обычное, прямоугольное. Крестик и системная
	// «Назад» выбрасывают снятое.
	let {
		settings,
		onsend,
		onclose
	}: {
		/** Настройки сжатия сервера (9.7): высота кадра и битрейт видео. */
		settings?: CompressionSettings;
		/** poster — JPEG кадра, снятый с камеры во время записи. */
		onsend: (file: File, poster?: ArrayBuffer) => void;
		onclose: () => void;
	} = $props();

	type Phase = 'live' | 'recording' | 'review';
	let phase = $state<Phase>('live');
	let facing = $state<'environment' | 'user'>('environment');
	let elapsedMs = $state(0);
	let error = $state('');
	// Имя отказа браузера (NotAllowedError…) — мелкой строкой под текстом:
	// по нему видно, что именно не дало камеру на этом устройстве.
	let errorCode = $state('');
	let reviewUrl = $state('');

	let liveEl: HTMLVideoElement | undefined = $state();
	let stream: MediaStream | null = null;
	let recorder: MediaRecorder | null = null;
	let chunks: Blob[] = [];
	let take: File | null = null;
	let takeSize = $state(0);
	// Размер кадра снятого — его знает сам ролик на экране просмотра.
	let takeFrame = $state<{ width: number; height: number } | undefined>();
	let startedAt = 0;
	let tick: ReturnType<typeof setInterval> | null = null;
	let releaseWake: (() => void) | null = null;

	// Камера пишет сразу тем, до чего видео сжимается по настройкам сервера
	// (9.7): та же высота кадра. Без кодировщика (Firefox на Android) ролик
	// уйдёт таким, каким записан, — тогда записи задаём и тот же битрейт.
	const canEncode = typeof VideoEncoder !== 'undefined';
	const short = $derived(settings?.video_max_height || DEFAULT_VIDEO_MAX_P);
	const long = $derived(Math.round((short * 16) / 9));
	const rawVideoBits = $derived((settings?.video_bitrate_kbps || DEFAULT_VIDEO_BITRATE_KBPS) * 1000);

	// Что уйдёт (4.26): сказать до отправки, а не чтобы человек узнал это из
	// ленты. Со сжатием — размер кадра после него; без сжатия — ещё и вес,
	// потому что ролик уйдёт как записан.
	const sendNote = $derived.by(() => {
		if (!takeFrame) return canEncode ? '' : `Без сжатия · ${formatBytes(takeSize)}`;
		if (!canEncode) {
			return `Без сжатия · ${takeFrame.width} × ${takeFrame.height} · ${formatBytes(takeSize)}`;
		}
		const out = targetVideoSize(takeFrame.width, takeFrame.height, short);
		return `${out.width} × ${out.height}`;
	});

	// Кадр для ленты — с живой камеры, пока идёт запись.
	let poster: ArrayBuffer | undefined;
	let opening = 0;

	/**
	 * Открыть камеру с одной просьбой и вернуть настоящий размер кадра. Его
	 * знает только само видео: настройки дорожки повторяют просьбу. Две камеры
	 * разом телефон не даёт — прежний поток гасим до запроса.
	 */
	async function openWith(
		ask: MediaTrackConstraints,
		turn: number
	): Promise<{ w: number; h: number } | { failed: string } | 'stale' | undefined> {
		stopStream();
		let next: MediaStream;
		try {
			next = await navigator.mediaDevices.getUserMedia({
				video: { facingMode: facing, ...ask },
				audio: true
			});
		} catch (err) {
			return { failed: err instanceof Error ? err.name : 'Error' };
		}
		if (turn !== opening) {
			next.getTracks().forEach((t) => t.stop());
			return 'stale';
		}
		stream = next;
		const video = liveEl;
		if (!video) return undefined;
		const ready = new Promise<void>((resolve) => {
			const timer = setTimeout(resolve, 3000);
			video.addEventListener(
				'loadedmetadata',
				() => {
					clearTimeout(timer);
					resolve();
				},
				{ once: true }
			);
		});
		video.srcObject = next;
		void video.play().catch(() => {});
		await ready;
		if (turn !== opening) return 'stale';
		return video.videoWidth ? { w: video.videoWidth, h: video.videoHeight } : undefined;
	}

	async function openCamera() {
		const turn = ++opening;
		error = '';
		errorCode = '';
		// Последняя просьба — без размеров кадра: на неё идём, только если
		// ни одна с размерами не открыла камеру (Firefox на планшете).
		const asks = [...cameraAsks(long, short), {}];
		let failed = '';
		let square: MediaTrackConstraints | undefined;
		for (const ask of asks) {
			if (ask === asks[asks.length - 1] && square) break;
			const got = await openWith(ask, turn);
			if (got === 'stale') return;
			if (got && 'failed' in got) {
				failed = got.failed;
				// Запрет другая просьба не снимет, только спросит человека заново.
				if (cameraFailure(failed).final) break;
				continue;
			}
			failed = '';
			// Не квадрат — годится. Квадрат — пробуем следующую просьбу.
			if (!got || got.w !== got.h) return;
			square = ask;
		}
		// После квадрата следующая просьба не прошла — возвращаемся к нему:
		// квадрат лучше, чем ничего.
		if (failed && square) {
			const got = await openWith(square, turn);
			if (got === 'stale') return;
			if (!(got && 'failed' in got)) return;
			failed = got.failed;
		}
		if (failed) {
			error = cameraFailure(failed).text;
			errorCode = failed;
		}
	}

	function stopStream() {
		stream?.getTracks().forEach((t) => t.stop());
		stream = null;
	}

	function flip() {
		facing = facing === 'environment' ? 'user' : 'environment';
		void openCamera();
	}

	function start() {
		if (!stream) return;
		const type = pickRecorderType('video');
		recorder = new MediaRecorder(stream, {
			...(type ? { mimeType: type } : {}),
			...(canEncode ? {} : { videoBitsPerSecond: rawVideoBits })
		});
		chunks = [];
		recorder.ondataavailable = (e) => {
			if (e.data.size) chunks.push(e.data);
		};
		recorder.onstop = finish;
		recorder.start(1000);
		startedAt = performance.now();
		elapsedMs = 0;
		phase = 'recording';
		releaseWake = holdWakeLock();
		poster = undefined;
		let grabbing = false;
		tick = setInterval(() => {
			elapsedMs = performance.now() - startedAt;
			if (elapsedMs >= VIDEO_MAX_MS) stop();
			// Первый нечёрный кадр записи; пока его нет — пробуем на каждом тике.
			if (!poster && !grabbing && liveEl) {
				grabbing = true;
				void liveFrameJpeg(liveEl).then((jpeg) => {
					poster ??= jpeg;
					grabbing = false;
				});
			}
		}, 200);
	}

	function stop() {
		if (recorder && recorder.state !== 'inactive') recorder.stop();
		if (tick) clearInterval(tick);
		tick = null;
		releaseWake?.();
		releaseWake = null;
	}

	function finish() {
		const type = recorder?.mimeType || pickRecorderType('video') || 'video/webm';
		const blob = new Blob(chunks, { type });
		chunks = [];
		recorder = null;
		stopStream();
		if (!blob.size) {
			error = 'Запись пустая';
			phase = 'live';
			void openCamera();
			return;
		}
		take = new File([blob], `Видео.${recordingExtension(type)}`, { type: blob.type });
		takeSize = blob.size;
		takeFrame = undefined;
		reviewUrl = URL.createObjectURL(blob);
		phase = 'review';
	}

	function retake() {
		if (reviewUrl) URL.revokeObjectURL(reviewUrl);
		reviewUrl = '';
		take = null;
		poster = undefined;
		phase = 'live';
		void openCamera();
	}

	function send() {
		if (!take) return;
		onsend(take, poster);
		close();
	}

	function close() {
		if (recorder && recorder.state !== 'inactive') {
			recorder.onstop = null;
			recorder.stop();
		}
		stop();
		stopStream();
		if (reviewUrl) URL.revokeObjectURL(reviewUrl);
		reviewUrl = '';
		onclose();
	}

	onMount(() => void openCamera());
	onDestroy(() => {
		stop();
		stopStream();
		if (reviewUrl) URL.revokeObjectURL(reviewUrl);
	});
</script>

<div class="vrec" role="dialog" aria-modal="true" aria-label="Запись видео" use:modal={{ ondismiss: close }}>
	{#if phase === 'review'}
		<!-- svelte-ignore a11y_media_has_caption -->
		<video
			class="vrec-view"
			src={reviewUrl}
			controls
			playsinline
			onloadedmetadata={(e) => {
				const { videoWidth: width, videoHeight: height } = e.currentTarget;
				if (width && height) takeFrame = { width, height };
			}}
		></video>
		<div class="vrec-top">
			<IconButton name="x" label="Закрыть" onclick={close} />
			<span class="vrec-timer">{sendNote}</span>
		</div>
		<div class="vrec-foot">
			<button type="button" class="vrec-text" onclick={retake}>Переснять</button>
			<span class="rec-time">{formatDuration(elapsedMs)}</span>
			<button type="button" class="vrec-send" onclick={send}>Отправить</button>
		</div>
	{:else}
		<video class="vrec-view" bind:this={liveEl} muted playsinline autoplay></video>
		<div class="vrec-top">
			<IconButton name="x" label="Закрыть" onclick={close} />
			{#if phase === 'recording'}
				<span class="vrec-timer">
					<span class="rec-dot"></span>
					<span class="rec-time">{formatDuration(elapsedMs)} / {formatDuration(VIDEO_MAX_MS)}</span>
				</span>
			{:else}
				<span class="vrec-timer"></span>
			{/if}
			<IconButton
				name="flip"
				label="Другая камера"
				disabled={phase === 'recording'}
				onclick={flip}
			/>
		</div>
		{#if error && phase === 'live'}
			<div class="vrec-note" role="alert">
				{error}
				{#if errorCode}<div class="vrec-code">{errorCode}</div>{/if}
			</div>
		{/if}
		<button
			type="button"
			class="vrec-shutter"
			class:on={phase === 'recording'}
			aria-label={phase === 'recording' ? 'Остановить запись' : 'Начать запись'}
			disabled={!!error && phase === 'live'}
			onclick={() => (phase === 'recording' ? stop() : start())}
		><i></i></button>
	{/if}
</div>

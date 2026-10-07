<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import { modal } from '$lib/a11y/modal';
	import { holdWakeLock } from '$lib/media/wake-lock';
	import {
		VIDEO_MAX_MS,
		formatDuration,
		pickRecorderType,
		recordingExtension,
		uprightFrame
	} from '$lib/media/record';

	// Запись видео (4.25) и просмотр перед отправкой (4.26). Камера во весь
	// экран, задняя по умолчанию; касание большой кнопки — старт и стоп;
	// предел — 5 минут. Видео обычное, прямоугольное. Крестик и системная
	// «Назад» выбрасывают снятое.
	let {
		onsend,
		onclose
	}: {
		onsend: (file: File) => void;
		onclose: () => void;
	} = $props();

	type Phase = 'live' | 'recording' | 'review';
	let phase = $state<Phase>('live');
	let facing = $state<'environment' | 'user'>('environment');
	let elapsedMs = $state(0);
	let error = $state('');
	let reviewUrl = $state('');

	let liveEl: HTMLVideoElement | undefined = $state();
	let stream: MediaStream | null = null;
	let recorder: MediaRecorder | null = null;
	let chunks: Blob[] = [];
	let take: File | null = null;
	let startedAt = 0;
	let tick: ReturnType<typeof setInterval> | null = null;
	let releaseWake: (() => void) | null = null;

	// Без кодировщика (Firefox на Android) ролик уйдёт таким, каким записан:
	// сжать его потом нечем. Тогда просим у камеры 720p и скромный битрейт.
	const canEncode = typeof VideoEncoder !== 'undefined';
	const RAW_VIDEO_BITS = 2_500_000;

	async function openCamera() {
		stopStream();
		error = '';
		const [long, short] = canEncode ? [1920, 1080] : [1280, 720];
		try {
			stream = await navigator.mediaDevices.getUserMedia({
				video: { facingMode: facing, width: { ideal: long }, height: { ideal: short } },
				audio: true
			});
		} catch {
			error = 'Нет доступа к камере';
			return;
		}
		const track = stream.getVideoTracks()[0];
		const upright = track
			? uprightFrame(
					track.getSettings(),
					long,
					short,
					window.matchMedia('(orientation: portrait)').matches
				)
			: undefined;
		if (upright) await track.applyConstraints(upright).catch(() => {});
		if (liveEl) {
			liveEl.srcObject = stream;
			void liveEl.play().catch(() => {});
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
			...(canEncode ? {} : { videoBitsPerSecond: RAW_VIDEO_BITS })
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
		tick = setInterval(() => {
			elapsedMs = performance.now() - startedAt;
			if (elapsedMs >= VIDEO_MAX_MS) stop();
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
		reviewUrl = URL.createObjectURL(blob);
		phase = 'review';
	}

	function retake() {
		if (reviewUrl) URL.revokeObjectURL(reviewUrl);
		reviewUrl = '';
		take = null;
		phase = 'live';
		void openCamera();
	}

	function send() {
		if (!take) return;
		onsend(take);
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
		<video class="vrec-view" src={reviewUrl} controls playsinline></video>
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
				<span class="vrec-timer">{error}</span>
			{/if}
			<IconButton
				name="flip"
				label="Другая камера"
				disabled={phase === 'recording'}
				onclick={flip}
			/>
		</div>
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

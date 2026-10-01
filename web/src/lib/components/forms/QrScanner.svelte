<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { decodeQrFrom } from '$lib/auth/links';

	// Видоискатель QR-кода (2.17; план 47, 3.8): задняя камера, кадр раз в
	// 250 мс. Что делать с прочитанным, решает экран: onread возвращает true —
	// хватит, камера гаснет; false — ждём следующий код. Камера гаснет и при
	// уходе с экрана.
	let {
		onread,
		onerror
	}: {
		onread: (text: string) => boolean;
		/** Камеры нет или её не дали — человеку предложат фото с кодом. */
		onerror: (message: string) => void;
	} = $props();

	let video: HTMLVideoElement | undefined = $state();
	let stream: MediaStream | undefined;
	let timer: ReturnType<typeof setTimeout> | undefined;
	let stopped = false;

	function stop() {
		stopped = true;
		clearTimeout(timer);
		stream?.getTracks().forEach((track) => track.stop());
		stream = undefined;
	}

	async function scan() {
		if (stopped || !video) return;
		if (video.readyState >= 2) {
			const text = await decodeQrFrom(video, video.videoWidth, video.videoHeight).catch(
				() => null
			);
			if (text && onread(text)) {
				stop();
				return;
			}
		}
		timer = setTimeout(() => void scan(), 250);
	}

	onMount(async () => {
		if (!navigator.mediaDevices?.getUserMedia) {
			onerror('Камера недоступна в этом браузере — выберите фото с кодом.');
			return;
		}
		try {
			stream = await navigator.mediaDevices.getUserMedia({
				video: { facingMode: { ideal: 'environment' } },
				audio: false
			});
		} catch (err) {
			onerror(
				err instanceof DOMException && err.name === 'NotAllowedError'
					? 'Нет доступа к камере — разрешите его в настройках браузера или выберите фото с кодом.'
					: 'Камера не включилась — выберите фото с кодом.'
			);
			return;
		}
		if (stopped || !video) {
			stop();
			return;
		}
		video.srcObject = stream;
		await video.play().catch(() => {});
		void scan();
	});

	onDestroy(stop);
</script>

<!-- svelte-ignore a11y_media_has_caption -->
<video class="qr-video" bind:this={video} playsinline muted></video>

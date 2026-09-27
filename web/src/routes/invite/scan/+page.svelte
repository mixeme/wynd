<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Hint from '$ui/forms/Hint.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { decodeQrFrom, inviteTarget } from '$lib/auth/links';

	// 2.17: QR-код приглашения показывают на чужом экране (2.7, 9.2) — значит,
	// считывать его логично здесь же, камерой.
	let video: HTMLVideoElement | undefined = $state();
	let stream: MediaStream | undefined;
	let timer: ReturnType<typeof setTimeout> | undefined;
	let stopped = false;
	let error = $state('');
	let notice = $state('');

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
			if (text) {
				const target = inviteTarget(text);
				if ('path' in target) {
					stop();
					goto(target.path);
					return;
				}
				// Чужой код — сказать и ждать следующего: камера остаётся включённой.
				notice = target.error;
			}
		}
		timer = setTimeout(() => void scan(), 250);
	}

	onMount(async () => {
		if (!navigator.mediaDevices?.getUserMedia) {
			error = 'Камера недоступна в этом браузере — выберите фото с кодом.';
			return;
		}
		try {
			stream = await navigator.mediaDevices.getUserMedia({
				video: { facingMode: { ideal: 'environment' } },
				audio: false
			});
		} catch (err) {
			error =
				err instanceof DOMException && err.name === 'NotAllowedError'
					? 'Нет доступа к камере — разрешите его в настройках браузера или выберите фото с кодом.'
					: 'Камера не включилась — выберите фото с кодом.';
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

<FormLayout shell app title="Сканер" onback={() => goto('/invite')}>
	<!-- svelte-ignore a11y_media_has_caption -->
	<video class="qr-video" bind:this={video} playsinline muted></video>
	{#if error}
		<Hint>{error}</Hint>
	{:else if notice}
		<Hint>{notice}</Hint>
	{:else}
		<Hint>Наведите камеру на QR-код приглашения — он откроется сам.</Hint>
	{/if}
</FormLayout>

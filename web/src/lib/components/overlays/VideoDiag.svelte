<script lang="ts">
	// Временная страница (план 46, C26): на iPhone ролик не играет в окне
	// просмотра, хотя в предпросмотре видеосообщения тот же файл играет. Здесь
	// один ролик пробуется несколькими способами — какой из них играет, тот и
	// покажет виновника. Убрать, когда причина найдена.
	import { onDestroy } from 'svelte';
	import { apiFetch } from '$lib/api/client';
	import { getMediaUrl } from '$lib/media/objectUrl';

	let {
		origin = '',
		blobId = '',
		posterId = '',
		link
	}: {
		origin?: string;
		blobId?: string;
		posterId?: string;
		/** Вместо страницы — кнопка-вход на неё (в окне просмотра ролика). */
		link?: string;
	} = $props();

	interface Variant {
		id: string;
		title: string;
		url?: string;
		poster?: string;
		full?: boolean;
		result: string;
	}

	let facts = $state<string[]>([]);
	let variants = $state<Variant[]>([]);
	let running = $state(false);
	let done = $state(false);
	const own: string[] = [];
	const els: Record<string, HTMLVideoElement> = {};

	onDestroy(() => own.forEach((url) => URL.revokeObjectURL(url)));

	function make(blob: Blob): string {
		const url = URL.createObjectURL(blob);
		own.push(url);
		return url;
	}

	function hex(bytes: Uint8Array): string {
		return Array.from(bytes, (b) => (b >= 32 && b < 127 ? String.fromCharCode(b) : '.')).join('');
	}

	const wait = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

	/** Средняя яркость кадра: чёрная картинка при идущем плеере — тоже отказ. */
	function brightness(video: HTMLVideoElement): string {
		try {
			const canvas = document.createElement('canvas');
			canvas.width = 32;
			canvas.height = 32;
			const ctx = canvas.getContext('2d');
			if (!ctx) return 'нет холста';
			ctx.drawImage(video, 0, 0, 32, 32);
			const data = ctx.getImageData(0, 0, 32, 32).data;
			let sum = 0;
			for (let i = 0; i < data.length; i += 4) sum += data[i] + data[i + 1] + data[i + 2];
			return String(Math.round(sum / (data.length / 4) / 3));
		} catch (err) {
			return `ошибка ${(err as Error).name}`;
		}
	}

	async function probe(variant: Variant): Promise<string> {
		const video = els[variant.id];
		if (!video) return 'нет элемента';
		const events: string[] = [];
		const names = ['loadstart', 'loadedmetadata', 'loadeddata', 'canplay', 'playing', 'stalled', 'suspend', 'waiting', 'error', 'abort', 'emptied'];
		const note = (e: Event) => {
			if (!events.includes(e.type)) events.push(e.type);
		};
		names.forEach((name) => video.addEventListener(name, note));
		video.muted = true;
		let play = 'ок';
		try {
			video.load();
			await video.play();
		} catch (err) {
			play = `${(err as Error).name}: ${(err as Error).message}`.slice(0, 80);
		}
		await wait(2500);
		names.forEach((name) => video.removeEventListener(name, note));
		const res =
			`play ${play} · ready ${video.readyState} · net ${video.networkState}` +
			` · ${video.videoWidth}×${video.videoHeight} · t ${video.currentTime.toFixed(2)} из ${Number.isFinite(video.duration) ? video.duration.toFixed(2) : video.duration}` +
			` · ошибка ${video.error ? `${video.error.code} ${video.error.message}`.slice(0, 60) : 'нет'}` +
			` · яркость ${brightness(video)}\n${events.join(' ')}`;
		video.pause();
		return res;
	}

	async function run() {
		running = true;
		done = false;
		facts = [];
		variants = [];
		const say = (line: string) => (facts = [...facts, line]);
		try {
			say(navigator.userAgent);
			say(`приложение с экрана «Домой»: ${matchMedia('(display-mode: standalone)').matches ? 'да' : 'нет'} · сервис-воркер: ${navigator.serviceWorker?.controller ? 'да' : 'нет'}`);

			const cachedUrl = await getMediaUrl(origin, blobId);
			const posterUrl = posterId ? await getMediaUrl(origin, posterId).catch(() => undefined) : undefined;

			const res = await apiFetch(origin, `/blobs/${blobId}`);
			const mime = res.headers.get('Content-Type') || '';
			const fresh = await res.blob();
			const buffer = await fresh.arrayBuffer();
			const base = mime.split(';')[0].trim();
			say(`сервер: ${mime || '—'} · ${fresh.size} байт · тип блоба «${fresh.type}»`);
			say(`начало файла: ${hex(new Uint8Array(buffer.slice(0, 40)))}`);
			const probeEl = document.createElement('video');
			say(`canPlayType: «${mime}» → «${probeEl.canPlayType(mime)}» · «${base}» → «${probeEl.canPlayType(base)}»`);

			variants = [
				{ id: 'a', title: 'А. Как в окне просмотра: кэш, обложка, preload, субтитры', url: cachedUrl, poster: posterUrl, full: true, result: '' },
				{ id: 'b', title: 'Б. Тот же адрес из кэша, голый плеер', url: cachedUrl, result: '' },
				{ id: 'c', title: 'В. Свежий ответ сервера как есть', url: make(fresh), result: '' },
				{ id: 'd', title: `Г. Свежие байты, тип «${base}» без параметров`, url: make(new Blob([buffer], { type: base })), result: '' },
				{ id: 'e', title: 'Д. Свежие байты, тип «video/mp4»', url: make(new Blob([buffer], { type: 'video/mp4' })), result: '' },
				{ id: 'f', title: 'Е. Свежие байты без типа', url: make(new Blob([buffer])), result: '' }
			];
			await wait(300);
			for (const variant of variants) {
				variant.result = 'проверяем…';
				variants = [...variants];
				variant.result = await probe(variant);
				variants = [...variants];
			}
			done = true;
		} catch (err) {
			say(`сбой диагностики: ${(err as Error).name} ${(err as Error).message}`);
		} finally {
			running = false;
		}
	}
</script>

{#if link}
	<a class="diag-link" href={link}>диагностика</a>
{:else}
<div class="diag">
	<p><b>Диагностика видео</b> · <a href="./?lb=0">назад</a></p>
	<button type="button" onclick={run} disabled={running || !blobId}>
		{running ? 'Идёт проверка…' : 'Проверить'}
	</button>
	{#each facts as line, i (i)}
		<p class="fact">{line}</p>
	{/each}
	{#each variants as variant (variant.id)}
		<div class="row">
			<p><b>{variant.title}</b></p>
			{#if variant.full}
				<video
					bind:this={els[variant.id]}
					src={variant.url}
					poster={variant.poster}
					controls
					playsinline
					preload="metadata"
				>
					<track kind="captions" label="Субтитры отсутствуют" />
				</video>
			{:else}
				<!-- svelte-ignore a11y_media_has_caption -->
				<video bind:this={els[variant.id]} src={variant.url} controls playsinline></video>
			{/if}
			<p class="fact">{variant.result}</p>
		</div>
	{/each}
	{#if done}
		<p><b>Готово.</b> Пришлите снимки этой страницы целиком (её можно прокрутить).</p>
	{/if}
</div>
{/if}

<style>
	.diag-link {
		position: fixed;
		z-index: 1000;
		right: 12px;
		bottom: 64px;
		padding: 8px 12px;
		border-radius: 8px;
		background: #fff;
		color: #000;
		font-size: 14px;
	}
	.diag {
		height: var(--app-h, 100dvh);
		overflow-y: auto;
		padding: 12px;
		background: #fff;
		color: #000;
		font: 13px/1.35 system-ui, sans-serif;
	}
	.diag p {
		margin: 6px 0;
	}
	.diag button {
		padding: 10px 16px;
		font-size: 16px;
	}
	.fact {
		font: 11px/1.3 ui-monospace, monospace;
		white-space: pre-wrap;
		word-break: break-all;
	}
	.row {
		border-top: 1px solid #999;
		padding: 6px 0;
	}
	video {
		display: block;
		width: 160px;
		height: 120px;
		background: #456;
	}
</style>

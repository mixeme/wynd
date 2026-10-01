<script lang="ts">
	import Icon, { type IconName } from '$ui/Icon.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import { audioTimeLabel } from '$lib/journal/present';
	import { formatBytes } from '$lib/format/bytes';
	import {
		audioPlayKey,
		audioTrack,
		subscribeAudio,
		toggleAudio,
		type AudioMeta
	} from '$lib/media/audioPlay';

	let {
		filename,
		size = '',
		class: className = '',
		style = '',
		onclick,
		audio = false,
		origin = '',
		blobId = '',
		coverUrl = '',
		onDownload,
		preview,
		meta,
		grouped = false,
		icon = 'file',
		strong = false
	}: {
		filename: string;
		size?: string;
		class?: string;
		style?: string;
		onclick?: () => void;
		audio?: boolean;
		origin?: string;
		blobId?: string;
		coverUrl?: string;
		onDownload?: () => void;
		/** Каталог библиотеки: рисунок без файла. */
		preview?: { progress: number; time: string };
		/** Для полосы плеера на других экранах (4.20): что играет и откуда. */
		meta?: AudioMeta;
		/** Несколько звуков записи — одна рамка, строки через черту (4.18). */
		grouped?: boolean;
		/** Значок файла: скриншот оплаты — фото (9.x), остальное — файл. */
		icon?: IconName;
		/** Имя жирным — когда строка не в записи, а сама предмет экрана. */
		strong?: boolean;
	} = $props();

	let revision = $state(0);

	$effect(() => {
		if (!audio || preview || !blobId) return;
		return subscribeAudio(() => {
			revision += 1;
		});
	});

	const track = $derived.by(() => {
		if (!audio || !blobId) return undefined;
		revision;
		return audioTrack(audioPlayKey(origin, blobId));
	});

	// Пока файл скачивается (4.19), полоска — сколько скачано, серым.
	const loading = $derived(preview ? undefined : track?.loading);
	const progress = $derived(
		preview
			? preview.progress
			: loading
				? loading.total
					? Math.min(1, loading.received / loading.total)
					: 0
				: track?.duration
					? Math.min(1, track.current / track.duration)
					: 0
	);
	const timeLabel = $derived(
		preview
			? preview.time
			: loading
				? loading.total
					? `загрузка · ${formatBytes(loading.received)} из ${formatBytes(loading.total)}`
					: 'загрузка…'
				: audioTimeLabel(Boolean(track?.started), track?.current ?? 0, track?.duration ?? 0)
	);
	const showPause = $derived(Boolean(track?.playing || loading));
	const barWidth = $derived(`${progress * 100}%`);

	function onClick(e: MouseEvent) {
		e.stopPropagation();
		onclick?.();
	}

	function onPlay(e: MouseEvent) {
		e.stopPropagation();
		// Свой сервер — origin ''. Проверка на истинность глушила звук целиком.
		if (!blobId) return;
		void toggleAudio(origin, blobId, meta);
	}
</script>

{#if audio}
	<div class="att audio {className}" class:grouped {style}>
		<button
			type="button"
			class="att-play"
			aria-pressed={showPause ? 'true' : 'false'}
			onclick={onPlay}
		>
			<div class="pic att-cover">
				{#if coverUrl}<img src={coverUrl} alt="" />{/if}
			</div>
			<div class="att-body">
				<div class="att-line">
					<Icon name={showPause ? 'pause' : 'play'} />
					<div class="att-name">{filename}</div>
				</div>
				<div class="att-bar" class:loading aria-hidden="true"><i style:width={barWidth}></i></div>
				<div class="sz">{timeLabel}</div>
			</div>
		</button>
		<IconButton name="download" label="Скачать" stopPropagation onclick={() => onDownload?.()} />
	</div>
{:else if onclick}
	<button type="button" class="att {className}" {style} onclick={onClick}>
		<Icon name={icon} />
		<div class="g" style:flex="1">
			<div style="font-size:12.5px" class:bold={strong}>{filename}</div>
			<div class="sz">{size}</div>
		</div>
	</button>
{:else}
	<div class="att {className}" {style}>
		<Icon name={icon} />
		<div class="g" style:flex="1">
			<div style="font-size:12.5px" class:bold={strong}>{filename}</div>
			<div class="sz">{size}</div>
		</div>
	</div>
{/if}

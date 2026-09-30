<script lang="ts">
	import { onDestroy } from 'svelte';
	import Icon from '$ui/Icon.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import { audioTimeLabel } from '$lib/journal/present';
	import {
		audioPlayKey,
		audioTrack,
		stopAudioIf,
		subscribeAudio,
		toggleAudio
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
		preview
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

	const progress = $derived(
		preview
			? preview.progress
			: track?.duration
				? Math.min(1, track.current / track.duration)
				: 0
	);
	const timeLabel = $derived(
		preview ? preview.time : audioTimeLabel(Boolean(track?.started), track?.current ?? 0, track?.duration ?? 0)
	);
	const barWidth = $derived(`${progress * 100}%`);

	function onClick(e: MouseEvent) {
		e.stopPropagation();
		onclick?.();
	}

	function onPlay(e: MouseEvent) {
		e.stopPropagation();
		// Свой сервер — origin ''. Проверка на истинность глушила звук целиком.
		if (!blobId) return;
		void toggleAudio(origin, blobId);
	}

	onDestroy(() => {
		if (audio && blobId) stopAudioIf(audioPlayKey(origin, blobId));
	});
</script>

{#if audio}
	<div class="att audio {className}" {style}>
		<button
			type="button"
			class="att-play"
			aria-pressed={track?.playing ? 'true' : 'false'}
			onclick={onPlay}
		>
			<div class="pic att-cover">
				{#if coverUrl}<img src={coverUrl} alt="" />{/if}
			</div>
			<div class="att-body">
				<div class="att-line">
					<Icon name={track?.playing ? 'pause' : 'play'} />
					<div class="att-name">{filename}</div>
				</div>
				<div class="att-bar" aria-hidden="true"><i style:width={barWidth}></i></div>
				<div class="sz">{timeLabel}</div>
			</div>
		</button>
		<IconButton name="download" label="Скачать" stopPropagation onclick={() => onDownload?.()} />
	</div>
{:else if onclick}
	<button type="button" class="att {className}" {style} onclick={onClick}>
		<Icon name="file" />
		<div class="g" style:flex="1">
			<div style="font-size:12.5px">{filename}</div>
			<div class="sz">{size}</div>
		</div>
	</button>
{:else}
	<div class="att {className}" {style}>
		<Icon name="file" />
		<div class="g" style:flex="1">
			<div style="font-size:12.5px">{filename}</div>
			<div class="sz">{size}</div>
		</div>
	</div>
{/if}

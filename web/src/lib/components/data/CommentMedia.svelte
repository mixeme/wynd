<script lang="ts">
	import Icon from '$ui/Icon.svelte';
	import VoiceRow from '$ui/data/VoiceRow.svelte';
	import VoiceWave from '$ui/data/VoiceWave.svelte';
	import { formatBytes } from '$lib/format/bytes';
	import type { CommentMediaItem } from '$lib/journal/present';
	import type { AudioMeta } from '$lib/media/audioPlay';
	import { formatDuration } from '$lib/media/record';

	// Вложения комментария (4.29): ряд маленьких плиток, голосовое и файл
	// строкой, без рамки — реплика остаётся на бумаге. Один вид в нити, в
	// превью под записью (4.30), в «Откликах» и у реплики в очереди.
	let {
		items,
		origin = '',
		audio,
		inert = false,
		onphoto,
		onfile
	}: {
		items: CommentMediaItem[];
		origin?: string;
		/** Для полосы плеера (4.20): откуда играет голосовое. */
		audio?: Omit<AudioMeta, 'title'>;
		/** Только вид, без нажатий: строка-ссылка «Откликов», реплика в очереди. */
		inert?: boolean;
		/** Касание снимка: его номер среди снимков реплики. */
		onphoto?: (index: number) => void;
		onfile?: (item: CommentMediaItem) => void;
	} = $props();

	const TILES = 3;
	const photos = $derived(items.filter((m) => m.kind === 'photo'));
	const rest = $derived(items.filter((m) => m.kind !== 'photo'));
	const extra = $derived(Math.max(0, photos.length - TILES));

	function tap(e: MouseEvent, fn: () => void) {
		e.stopPropagation();
		fn();
	}
</script>

{#snippet tile(item: CommentMediaItem, i: number)}
	{#if item.url}<img src={item.url} alt="" />{/if}
	{#if extra && i === TILES - 1}<span class="ctile-more">+{extra}</span>{/if}
{/snippet}

{#if items.length}
	<div class="cmedia">
		{#if photos.length}
			<div class="ctiles">
				{#each photos.slice(0, TILES) as item, i (item.key)}
					{#if inert || !onphoto}
						<span class="pic ctile">{@render tile(item, i)}</span>
					{:else}
						<button
							type="button"
							class="pic ctile"
							aria-label="Открыть фото"
							onclick={(e) => tap(e, () => onphoto(i))}
						>
							{@render tile(item, i)}
						</button>
					{/if}
				{/each}
			</div>
		{/if}
		{#each rest as item (item.key)}
			{#if item.kind === 'voice'}
				{#if inert || !item.blobId}
					<span class="voice">
						<span class="voice-play"><Icon name="play" /></span>
						<VoiceWave peaks={item.peaks?.length ? item.peaks : Array.from({ length: 32 }, () => 20)} />
						<span class="sz">{formatDuration(item.durationMs ?? 0)}</span>
					</span>
				{:else}
					<VoiceRow
						class="m-{item.blobId}"
						{origin}
						blobId={item.blobId}
						peaks={item.peaks ?? []}
						durationMs={item.durationMs ?? 0}
						meta={audio ? { ...audio, title: 'Голосовое' } : undefined}
					/>
				{/if}
			{:else if inert || !onfile}
				<span class="crow">
					<Icon name="file" />
					<span class="nm">{item.name}</span>
					{#if item.size}<span class="sz">{formatBytes(item.size)}</span>{/if}
				</span>
			{:else}
				<button type="button" class="crow" onclick={(e) => tap(e, () => onfile(item))}>
					<Icon name="file" />
					<span class="nm">{item.name}</span>
					{#if item.size}<span class="sz">{formatBytes(item.size)}</span>{/if}
					<Icon name="download" />
				</button>
			{/if}
		{/each}
	</div>
{/if}

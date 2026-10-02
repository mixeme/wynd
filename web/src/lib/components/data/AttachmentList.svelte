<script lang="ts">
	import AttachmentRow from '$ui/data/AttachmentRow.svelte';
	import { formatBytes } from '$lib/format/bytes';
	import {
		attachmentBlocks,
		attachmentLabel,
		attachmentSizeLabel,
		audioRowLabel
	} from '$lib/journal/present';
	import type { MediaSummary } from '$lib/journal/types';
	import { downloadBlob } from '$lib/media/objectUrl';

	// Вложения записи (план 47, 2.5 и 3.2): звуки подряд — одна рамка со
	// строками через черту (4.18), одиночный звук — как на 4.15, остальные
	// файлы — строкой «скачать». Блок был скопирован в ленту и на экран записи.
	let {
		items,
		origin,
		circleId,
		circleName,
		color,
		postId,
		coverUrls
	}: {
		/** Вложения записи по порядку (не фото и не видео). */
		items: MediaSummary[];
		origin: string;
		circleId: string;
		circleName: string;
		/** Цвет круга: линия хода на полосе плеера (4.20). */
		color: string;
		postId: string;
		/** Готовые адреса обложек звуков по blob id. */
		coverUrls: Record<string, string>;
	} = $props();

	const blocks = $derived(attachmentBlocks(items));

	function cover(att: MediaSummary): string | undefined {
		return att.audio_cover_blob_id ? coverUrls[att.audio_cover_blob_id] : undefined;
	}

	function download(att: MediaSummary) {
		void downloadBlob(origin, att.blob_id, attachmentLabel(att));
	}
</script>

{#snippet audioRow(att: MediaSummary, grouped: boolean)}
	<AttachmentRow
		audio
		{grouped}
		class="m-{att.blob_id}"
		filename={audioRowLabel(att)}
		{origin}
		blobId={att.blob_id}
		coverUrl={cover(att) ?? ''}
		meta={{ title: audioRowLabel(att), coverUrl: cover(att), circleId, circleName, color, postId }}
		onDownload={() => download(att)}
	/>
{/snippet}

{#each blocks as block, bi (bi)}
	{#if block.kind === 'audio'}
		{#if block.items.length > 1}
			<div class="att-group">
				{#each block.items as att (att.blob_id)}
					{@render audioRow(att, true)}
				{/each}
			</div>
		{:else}
			{@render audioRow(block.items[0], false)}
		{/if}
	{:else}
		<AttachmentRow
			class="m-{block.item.blob_id}"
			filename={attachmentLabel(block.item)}
			size={attachmentSizeLabel(block.item, formatBytes)}
			onclick={() => download(block.item)}
		/>
	{/if}
{/each}

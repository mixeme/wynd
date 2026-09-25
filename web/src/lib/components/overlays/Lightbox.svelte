<script lang="ts">
	import Icon from '$ui/Icon.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import type { Snippet } from 'svelte';

	let {
		counter,
		caption,
		media,
		dots,
		dotCount,
		dotIndex,
		onDotSelect,
		fixed = false,
		class: className = '',
		style = '',
		onclose,
		ondownload,
		onprev,
		onnext
	}: {
		counter?: string;
		caption?: string;
		media: Snippet;
		dots?: Snippet;
		dotCount?: number;
		dotIndex?: number;
		onDotSelect?: (index: number) => void;
		fixed?: boolean;
		class?: string;
		style?: string;
		onclose?: () => void;
		ondownload?: () => void;
		onprev?: () => void;
		onnext?: () => void;
	} = $props();

	let touchStartX = 0;

	function onKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') onclose?.();
		else if (e.key === 'ArrowLeft') onprev?.();
		else if (e.key === 'ArrowRight') onnext?.();
	}

	function onTouchStart(e: TouchEvent) {
		touchStartX = e.changedTouches[0].clientX;
	}

	function onTouchEnd(e: TouchEvent) {
		const dx = e.changedTouches[0].clientX - touchStartX;
		if (Math.abs(dx) < 40) return;
		if (dx > 0) onprev?.();
		else onnext?.();
	}
</script>

<svelte:window onkeydown={onKeydown} />

<div class="lb {className}" class:fixed {style}>
	<div class="top">
		{#if onclose}
			<IconButton name="x" label="Закрыть" onclick={() => onclose()} />
		{/if}
		{#if counter}
			<span style="font-size:13.5px">{counter}</span>
		{/if}
		{#if ondownload}
			<IconButton
				name="download"
				label="Скачать"
				onclick={() => ondownload()}
				style="margin-left:auto;color:#EFE9E0"
			/>
		{/if}
	</div>
	<div
		class="mid"
		role="region"
		aria-label="Просмотр"
		ontouchstart={onTouchStart}
		ontouchend={onTouchEnd}
	>
		{#if onprev}
			<button
				type="button"
				class="nav prev"
				aria-label="Предыдущее фото"
				onclick={() => onprev()}
			>
				<Icon name="chevr" style="transform:rotate(180deg)" />
			</button>
		{/if}
		{@render media()}
		{#if onnext}
			<button type="button" class="nav next" aria-label="Следующее фото" onclick={() => onnext()}>
				<Icon name="chevr" />
			</button>
		{/if}
	</div>
	{#if dots}
		<div class="dots">
			{@render dots()}
		</div>
	{:else if dotCount != null && onDotSelect}
		<div class="dots">
			{#each Array.from({ length: dotCount }, (_, i) => i) as i (i)}
				<button
					type="button"
					class:on={i === dotIndex}
					aria-label="Фото {i + 1}"
					onclick={() => onDotSelect(i)}
				></button>
			{/each}
		</div>
	{/if}
	{#if caption}
		<div class="cap">{caption}</div>
	{/if}
</div>

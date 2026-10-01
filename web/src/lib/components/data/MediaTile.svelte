<script lang="ts">
	import Icon from '$ui/Icon.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import { coverRectStyle, isCoverRect, type CoverRect } from '$lib/media/crop';

	type MediaKind = 'photo' | 'video';

	type Feed = {
		variant: 'feed';
		src?: string;
		kind?: MediaKind;
		count?: number;
		locationLabel?: string;
		/** Кадр обложки (4.16): лента рисует этот квадрат снимка. */
		crop?: CoverRect;
		onclick?: () => void;
	};

	type Grid = {
		variant: 'grid';
		src?: string;
		count?: number;
		onclick: () => void;
	};

	type Album = {
		variant: 'album';
		src?: string;
		kind?: MediaKind;
		coverLabel?: string;
		selected?: boolean;
		onclick: () => void;
	};

	type HeaderMini = {
		variant: 'headerMini';
		src: string;
		kind?: MediaKind;
		'aria-label': string;
		onclick: () => void;
	};

	type Compose = {
		variant: 'compose';
		src?: string;
		kind?: MediaKind;
		isCover?: boolean;
		/** Кадр обложки: плитка показывает то, что увидит лента. */
		crop?: CoverRect;
		/** Метка на плитке-обложке — дверь в кадр (4.2 → 4.16). */
		coverMark?: string;
		fileName?: string;
		onclick: () => void;
		onremove?: () => void;
	};

	let props: Feed | Grid | Album | HeaderMini | Compose = $props();

	const cropStyle = $derived.by(() => {
		const crop = props.variant === 'feed' || props.variant === 'compose' ? props.crop : undefined;
		return isCoverRect(crop) ? coverRectStyle(crop) : undefined;
	});

	function onFeedClick(e: MouseEvent) {
		if (props.variant !== 'feed' || !props.onclick) return;
		e.stopPropagation();
		props.onclick();
	}
</script>

{#if props.variant === 'feed'}
	<div
		class="pic sq"
		role="presentation"
		style={!props.src ? 'background:var(--tint)' : undefined}
		onclick={onFeedClick}
	>
		{#if props.src}
			{#if props.kind === 'video'}
				<video src={props.src} muted playsinline></video>
			{:else if cropStyle}
				<img
					class="cropped"
					src={props.src}
					alt=""
					style:width={cropStyle.width}
					style:height={cropStyle.height}
					style:left={cropStyle.left}
					style:top={cropStyle.top}
				/>
			{:else}
				<img src={props.src} alt="" />
			{/if}
		{/if}
		{#if props.locationLabel}
			<span class="tagr"><Icon name="loc" size="xs" />{props.locationLabel}</span>
		{/if}
		{#if props.count != null && (props.src ? props.count > 1 : true)}
			<span class="cnt">{props.count}</span>
		{/if}
	</div>
{:else if props.variant === 'grid'}
	<button type="button" class="pic" onclick={() => props.onclick()}>
		{#if props.src}
			<img src={props.src} alt="" />
		{/if}
		{#if props.count != null && props.count > 1}
			<span class="cnt">{props.count}</span>
		{/if}
	</button>
{:else if props.variant === 'album'}
	<button
		type="button"
		class="cell"
		class:on={props.selected}
		onclick={() => props.onclick()}
	>
		{#if props.src}
			{#if props.kind === 'video'}
				<video src={props.src} muted playsinline></video>
			{:else}
				<img src={props.src} alt="" />
			{/if}
		{/if}
		{#if props.coverLabel}
			<span class="cov">{props.coverLabel}</span>
		{/if}
		{#if props.selected}
			<span class="mark">✓</span>
		{/if}
	</button>
{:else if props.variant === 'headerMini'}
	<button
		type="button"
		class="pic sq mini"
		aria-label={props['aria-label']}
		onclick={() => props.onclick()}
	>
		{#if props.kind === 'video'}
			<video src={props.src} muted playsinline></video>
		{:else}
			<img src={props.src} alt="" />
		{/if}
	</button>
{:else}
	<div class="thumb" class:cover={props.isCover}>
		<button type="button" class="thumb-body" onclick={() => props.onclick()}>
			{#if props.src}
				{#if props.kind === 'video'}
					<video src={props.src} muted playsinline></video>
				{:else if cropStyle}
					<img
						class="cropped"
						src={props.src}
						alt=""
						style:width={cropStyle.width}
						style:height={cropStyle.height}
						style:left={cropStyle.left}
						style:top={cropStyle.top}
					/>
				{:else}
					<img src={props.src} alt="" />
				{/if}
			{:else if props.fileName}
				<span class="file">{props.fileName}</span>
			{/if}
			{#if props.variant === 'compose' && props.coverMark}
				<span class="cnt thumb-mark">{props.coverMark}</span>
			{/if}
		</button>
		{#if props.onremove}
			<IconButton
				name="x"
				size="sm"
				label="Убрать"
				class="thumb-rm"
				onclick={() => props.onremove!()}
			/>
		{/if}
	</div>
{/if}

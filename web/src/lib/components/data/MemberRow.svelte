<script lang="ts">
	import Avatar from '$ui/data/Avatar.svelte';
	import Row from '$ui/data/Row.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';

	let {
		initial,
		name,
		subtitle,
		color,
		src,
		menu = false,
		faded = false,
		onmenu,
		onclick,
		class: className = '',
		style = ''
	}: {
		initial: string;
		name: string;
		subtitle?: string;
		color?: string;
		src?: string;
		menu?: boolean;
		faded?: boolean;
		onmenu?: () => void;
		onclick?: () => void;
		class?: string;
		style?: string;
	} = $props();
</script>

<Row {onclick} opacity={faded ? 0.6 : undefined} class={className} {style}>
	{#snippet leading()}<Avatar {initial} {color} {src} />{/snippet}
	{#snippet main()}
		<div style="font-weight:600">{name}</div>
		{#if subtitle}
			<div class="sub">{subtitle}</div>
		{/if}
	{/snippet}
	{#snippet trailing()}
		<!-- Меню — только у строки, которая сама не нажимается: вложенных кнопок нет. -->
		{#if !onclick && menu && onmenu}
			<IconButton name="dots" size="sm" label="Меню" stopPropagation onclick={() => onmenu()} />
		{/if}
	{/snippet}
</Row>

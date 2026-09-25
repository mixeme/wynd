<script lang="ts">
	import Avatar from '$ui/data/Avatar.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';

	let {
		initial,
		name,
		subtitle,
		color,
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
		menu?: boolean;
		faded?: boolean;
		onmenu?: () => void;
		onclick?: () => void;
		class?: string;
		style?: string;
	} = $props();
</script>

{#if onclick}
	<button
		type="button"
		class="row2 {className}"
		style:opacity={faded ? 0.6 : undefined}
		{style}
		{onclick}
	>
		<Avatar {initial} {color} />
		<div class="g">
			<div style="font-weight:600">{name}</div>
			{#if subtitle}
				<div class="sub">{subtitle}</div>
			{/if}
		</div>
	</button>
{:else}
	<div class="row2 {className}" style:opacity={faded ? 0.6 : undefined} {style}>
		<Avatar {initial} {color} />
		<div class="g">
			<div style="font-weight:600">{name}</div>
			{#if subtitle}
				<div class="sub">{subtitle}</div>
			{/if}
		</div>
		{#if menu && onmenu}
			<IconButton
				name="dots"
				size="sm"
				label="Меню"
				stopPropagation
				onclick={() => onmenu()}
			/>
		{/if}
	</div>
{/if}

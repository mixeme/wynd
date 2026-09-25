<script lang="ts">
	import Avatar from '$ui/data/Avatar.svelte';
	import IconButton from '$ui/forms/IconButton.svelte';
	import type { Snippet } from 'svelte';

	let {
		initial,
		name,
		color,
		src,
		queued = false,
		time,
		children,
		onedit,
		ondelete,
		class: className = '',
		style = ''
	}: {
		initial: string;
		name: string;
		color?: string;
		src?: string;
		queued?: boolean;
		time: Snippet;
		children: Snippet;
		onedit?: () => void;
		ondelete?: () => void;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="cmt {className}" class:q={queued} {style}>
	<Avatar {initial} {color} {src} />
	<div class="g">
		<div class="who">
			<b>{name}</b>
			<span class="tm">{@render time()}</span>
		</div>
		{@render children()}
	</div>
	<div class="acts">
		{#if onedit}
			<IconButton name="edit" label="Править" size="sm" onclick={() => onedit()} />
		{/if}
		{#if ondelete}
			<IconButton name="trash" label="Удалить" size="sm" onclick={() => ondelete()} />
		{/if}
	</div>
</div>

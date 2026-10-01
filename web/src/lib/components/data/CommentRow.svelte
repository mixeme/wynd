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
		bare = false,
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
		/** Без колонки действий: строка-ссылка («Отклики»), не тред. В треде
		 *  колонка стоит всегда — текст одной ширины у своих и чужих. */
		bare?: boolean;
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
	{#if !bare}
		<div class="acts">
			{#if onedit}
				<IconButton name="edit" label="Править" size="sm" onclick={() => onedit()} />
			{/if}
			{#if ondelete}
				<IconButton name="trash" label="Удалить" size="sm" onclick={() => ondelete()} />
			{/if}
		</div>
	{/if}
</div>

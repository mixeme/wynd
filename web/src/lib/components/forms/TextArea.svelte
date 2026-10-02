<script lang="ts">
	import type { HTMLTextareaAttributes } from 'svelte/elements';
	import { splitMentionBody } from '$lib/journal/mentions';

	let {
		value = $bindable(''),
		variant = 'area',
		active = false,
		class: className = '',
		style = '',
		el = $bindable<HTMLTextAreaElement>(),
		...rest
	}: {
		value?: string;
		variant?: 'area' | 'field' | 'compose' | 'comment' | 'commentEdit';
		active?: boolean;
		class?: string;
		style?: string;
		el?: HTMLTextAreaElement;
	} & HTMLTextareaAttributes = $props();

	const rootClass = $derived(
		variant === 'field'
			? 'fld'
			: variant === 'compose'
				? 'compose-text compose-overlay'
				: variant === 'comment'
					? 'inp'
					: variant === 'commentEdit'
						? 'fld ced'
						: 'ta'
	);

	const mentionParts = $derived(variant === 'compose' ? splitMentionBody(value) : []);
</script>

{#if variant === 'compose'}
	<div class="compose-field">
		<div class="compose-mirror compose-text" aria-hidden="true">
			{#each mentionParts as part, i (i)}
				{#if part.kind === 'mention'}
					<span class="men">{part.value}</span>
				{:else}
					{part.value}
				{/if}
			{/each}{'\u200b'}
		</div>
		<textarea
			class="{rootClass} {className}"
			class:act={active}
			{style}
			bind:this={el}
			bind:value
			{...rest}
		></textarea>
	</div>
{:else}
	<textarea
		class="{rootClass} {className}"
		class:act={active}
		{style}
		bind:this={el}
		bind:value
		{...rest}
	></textarea>
{/if}

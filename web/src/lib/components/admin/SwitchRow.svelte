<script lang="ts">
	import type { Snippet } from 'svelte';
	import Switch from '$ui/forms/Switch.svelte';

	// Переключатель админки с заголовком и пояснением (кадры 9.x, 10.x; план
	// 47, 7.25): «Вход открыт», «Требовать подписку», «Показывать». Был собран
	// руками в трёх экранах.
	let {
		checked = $bindable(false),
		title,
		end = false,
		disabled = false,
		onchange,
		children,
		class: className = ''
	}: {
		checked?: boolean;
		title: string;
		/** Переключатель справа от текста (9.4), а не слева. */
		end?: boolean;
		disabled?: boolean;
		/** Только нажатие человеком — как у Switch. */
		onchange?: (checked: boolean) => void;
		/** Пояснение под заголовком. */
		children?: Snippet;
		class?: string;
	} = $props();
</script>

<div class="flex-top {end ? 'gap-20' : 'gap-12'} {className}">
	{#if !end}
		<Switch bind:checked {disabled} {onchange} class="mt-2" label={title} />
	{/if}
	<div class={end ? 'grow' : undefined}>
		<div class="ttl">{title}</div>
		{#if children}
			<div class="note mt-4 lh-15">{@render children()}</div>
		{/if}
	</div>
	{#if end}
		<Switch bind:checked {disabled} {onchange} class="mt-2" label={title} />
	{/if}
</div>

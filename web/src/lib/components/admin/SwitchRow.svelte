<script lang="ts">
	import type { Snippet } from 'svelte';
	import Switch from '$ui/forms/Switch.svelte';

	// Переключатель админки с заголовком и пояснением (кадры 9.x, 10.x; план
	// 47, 2.19): «Вход открыт», «Требовать подписку», «Показывать». Был собран
	// руками в трёх экранах.
	let {
		checked = $bindable(false),
		title,
		children,
		class: className = ''
	}: {
		checked?: boolean;
		title: string;
		/** Пояснение под заголовком. */
		children?: Snippet;
		class?: string;
	} = $props();
</script>

<div class="flex-top gap-12 {className}">
	<Switch bind:checked class="mt-2" label={title} />
	<div>
		<div class="ttl">{title}</div>
		{#if children}
			<div class="note mt-4 lh-15">{@render children()}</div>
		{/if}
	</div>
</div>

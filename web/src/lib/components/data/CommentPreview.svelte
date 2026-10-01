<script lang="ts">
	import type { Snippet } from 'svelte';

	// Комментарии под карточкой в ленте (3.1): первый — с временем, ниже «ещё N».
	// Нажатие ведёт в обсуждение. Строки собирает сам (план 47, 1.8): экрану
	// незачем знать про .tm и .mo внутри.
	let {
		first,
		time,
		more,
		children,
		onclick
	}: {
		/** «Миша: Даже не позорное качество…». */
		first?: string;
		/** Время первого: «вчера, 22:29». */
		time?: string;
		/** «ещё 4 комментария». */
		more?: string;
		/** Своё содержимое вместо строк (кадры /dev). */
		children?: Snippet;
		onclick: () => void;
	} = $props();
</script>

<button type="button" class="cm" {onclick}>
	{#if children}
		{@render children()}
	{:else}
		{#if first}
			<div>{first}{#if time}<span class="tm">{' · '}{time}</span>{/if}</div>
		{/if}
		{#if more}
			<div class="mo">{more}</div>
		{/if}
	{/if}
</button>

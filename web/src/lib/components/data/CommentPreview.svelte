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
		media,
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
		/** Вложения последней реплики (4.30): у них свои нажатия, поэтому рамка
		 *  тогда не кнопка — кнопками остаются строки. */
		media?: Snippet;
		onclick: () => void;
	} = $props();
</script>

{#snippet lines()}
	{#if first}
		<div>{first}{#if time}<span class="tm">{' · '}{time}</span>{/if}</div>
	{/if}
{/snippet}

{#if media && !children}
	<div class="cm">
		{#if first}
			<button type="button" class="cm-line" {onclick}>{@render lines()}</button>
		{/if}
		{@render media()}
		{#if more}
			<button type="button" class="cm-line mo" {onclick}>{more}</button>
		{/if}
	</div>
{:else}
	<button type="button" class="cm" {onclick}>
		{#if children}
			{@render children()}
		{:else}
			{@render lines()}
			{#if more}
				<div class="mo">{more}</div>
			{/if}
		{/if}
	</button>
{/if}

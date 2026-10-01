<script lang="ts">
	import type { Snippet } from 'svelte';
	import Icon, { type IconName } from '$ui/Icon.svelte';
	import CommentRow from '$ui/data/CommentRow.svelte';

	// Строка «Откликов» (3.13; план 47, 3.1): кто и что сделал — ссылкой туда,
	// где отклик живёт. Под словами экран ставит текст комментария и PostRef.
	let {
		href,
		onopen,
		initial,
		name,
		color,
		src,
		icon,
		label,
		children
	}: {
		href: string;
		/** Переход внутри приложения; href остаётся для «открыть в новой вкладке». */
		onopen: (e: MouseEvent) => void;
		initial: string;
		name: string;
		color?: string;
		src?: string;
		/** Знак реакции перед подписью. */
		icon?: IconName;
		/** «комментарий · 14:02». */
		label: string;
		children: Snippet;
	} = $props();
</script>

<a class="resp" {href} onclick={onopen}>
	<CommentRow {initial} {name} {color} {src} bare>
		{#snippet time()}
			<span class="resp-kind">{#if icon}<Icon name={icon} size="xs" />{/if}{label}</span>
		{/snippet}
		{@render children()}
	</CommentRow>
</a>

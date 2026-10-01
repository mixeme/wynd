<script lang="ts">
	// Рамка «к чему это» под откликом (3.13; план 47, 3.1): обложка записи,
	// автор, дата и начало текста одной строкой.
	let {
		author,
		date,
		excerpt,
		cover = false,
		coverUrl
	}: {
		/** «ваша запись» или имя автора — жирным. */
		author: string;
		/** День записи, уже словами. */
		date: string;
		excerpt?: string;
		/** У записи есть обложка — место под неё есть сразу, картинка доедет. */
		cover?: boolean;
		coverUrl?: string;
	} = $props();
</script>

<div class="post-ref">
	{#if cover}
		<span class="pic post-ref-pic">{#if coverUrl}<img src={coverUrl} alt="" />{/if}</span>
	{/if}
	<span class="post-ref-line"
		><b>{author}</b> · {date}{#if excerpt}{' · '}{excerpt}{/if}</span
	>
</div>

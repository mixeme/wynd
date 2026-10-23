<script lang="ts">
	import Icon from '$ui/Icon.svelte';

	// Вложения комментария при правке (владелец, 2026-10-23, план 46, C53): те
	// же плитки с крестиком, что под строкой ввода до отправки, и «+», пока
	// есть место. Снимок — миниатюрой, голосовое и файл — строкой с именем.
	let {
		items,
		onremove,
		onadd
	}: {
		items: { key: string; url?: string; name: string }[];
		onremove: (index: number) => void;
		/** Нет — места больше нет, «+» не показывается. */
		onadd?: () => void;
	} = $props();
</script>

<div class="cstrip cstrip-edit">
	{#each items as item, i (item.key)}
		<div class="cpend" class:file={!item.url}>
			{#if item.url}
				<img src={item.url} alt="" />
			{:else}
				<Icon name="file" /><span class="nm">{item.name}</span>
			{/if}
			<button type="button" class="cpend-rm" aria-label="Убрать" onclick={() => onremove(i)}>
				<Icon name="x" />
			</button>
		</div>
	{/each}
	{#if onadd}
		<button type="button" class="cpend-add" aria-label="Добавить вложение" onclick={onadd}>
			<Icon name="plus" />
		</button>
	{/if}
</div>

<script lang="ts">
	import Icon from '$ui/Icon.svelte';

	// Вложения комментария при правке (владелец, 2026-10-23, план 46, C53): те
	// же плитки с крестиком, что под строкой ввода до отправки, и «+», пока
	// есть место. Снимок — миниатюрой, голосовое и файл — строкой с именем.
	// Добавить — двумя плитками, как кнопками строки ввода: снимок открывает
	// галерею, файл — выбор файлов (одна «+» на все файлы открывала на Android
	// не тот выбор, что кнопка снимка).
	let {
		items,
		onremove,
		onaddphoto,
		onaddfile
	}: {
		items: { key: string; url?: string; name: string }[];
		onremove: (index: number) => void;
		/** Нет — места больше нет, плитки не показываются. */
		onaddphoto?: () => void;
		onaddfile?: () => void;
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
	{#if onaddphoto}
		<button type="button" class="cpend-add" aria-label="Добавить фото" onclick={onaddphoto}>
			<Icon name="plus" size="xs" /><Icon name="photo" />
		</button>
	{/if}
	{#if onaddfile}
		<button type="button" class="cpend-add" aria-label="Добавить файл" onclick={onaddfile}>
			<Icon name="plus" size="xs" /><Icon name="file" />
		</button>
	{/if}
</div>

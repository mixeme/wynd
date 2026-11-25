<script lang="ts">
	import Icon from '$ui/Icon.svelte';

	export type ReactionGroup = {
		emoji: string;
		names: string;
	};

	export type ReactionOption = {
		/** Ключ сервера: его отправляем при выборе. */
		key: string;
		emoji: string;
		/** Подпись для чтения с экрана. */
		label: string;
	};

	let {
		groups,
		keys,
		showAdd = false,
		pickerOpen = false,
		selectedKey = '',
		onopenList,
		onadd,
		onpick
	}: {
		groups: ReactionGroup[];
		keys: readonly ReactionOption[];
		showAdd?: boolean;
		pickerOpen?: boolean;
		selectedKey?: string;
		onopenList: () => void;
		onadd: () => void;
		onpick: (key: string) => void;
	} = $props();
</script>

<div class="rx">
	{#each groups as group, i (i)}
		<button type="button" class="one" onclick={() => onopenList()}>
			<span class="rxe">{group.emoji}</span>
			{group.names}
		</button>
	{/each}
	{#if showAdd}
		<button type="button" class="add" aria-label="Добавить реакцию" onclick={() => onadd()}>
			<Icon name="plus" size="xs" />
		</button>
	{/if}
</div>
{#if pickerOpen}
	<div class="rxpick">
		{#each keys as key (key.key)}
			<button
				type="button"
				class="rcho"
				class:on={selectedKey === key.key}
				aria-label={key.label}
				onclick={() => onpick(key.key)}
			>
				<span class="rxe">{key.emoji}</span>
			</button>
		{/each}
	</div>
{/if}

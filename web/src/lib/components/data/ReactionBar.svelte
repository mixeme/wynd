<script lang="ts">
	import Icon, { type IconName } from '$ui/Icon.svelte';

	export type ReactionGroup = {
		icon: IconName;
		names: string;
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
		keys: readonly IconName[];
		showAdd?: boolean;
		pickerOpen?: boolean;
		selectedKey?: string;
		onopenList: () => void;
		onadd: () => void;
		onpick: (key: IconName) => void;
	} = $props();
</script>

<div class="rx">
	{#each groups as group, i (i)}
		<button type="button" class="one" onclick={() => onopenList()}>
			<Icon name={group.icon} size="xs" style="color:var(--c)" />
			{group.names}
		</button>
	{/each}
	{#if showAdd}
		<button type="button" class="add" onclick={() => onadd()}>+</button>
	{/if}
</div>
{#if pickerOpen}
	<div class="rxpick">
		{#each keys as key (key)}
			<button
				type="button"
				class="rcho"
				class:on={selectedKey === key}
				onclick={() => onpick(key)}
			>
				<Icon name={key} />
			</button>
		{/each}
	</div>
{/if}

<script lang="ts">
	import IconButton from '$ui/forms/IconButton.svelte';
	import Mark from '$ui/Mark.svelte';

	let {
		onsearch,
		searchDisabled = false,
		onsettings
	}: {
		onsearch?: () => void;
		searchDisabled?: boolean;
		onsettings?: () => void;
	} = $props();
</script>

<div class="appbar">
	<Mark />
	<span class="sp"></span>
	{#if onsearch}
		<IconButton name="search" label="Поиск" onclick={() => onsearch()} />
	{:else if searchDisabled}
		<IconButton
			name="search"
			label="Поиск недоступен"
			disabled
			style="opacity:.35"
			onclick={() => {}}
		/>
	{/if}
	{#if onsettings}
		<IconButton name="gear" label="Настройки" onclick={() => onsettings()} />
	{/if}
</div>

<script lang="ts">
	import IconButton from '$ui/forms/IconButton.svelte';
	import Mark from '$ui/Mark.svelte';

	let {
		onsearch,
		onsettings
	}: {
		onsearch?: () => void;
		onsettings?: () => void;
	} = $props();
</script>

<div class="appbar">
	<Mark />
	<span class="sp"></span>
	{#if onsearch}
		<IconButton name="search" label="Поиск" onclick={() => onsearch()} />
	{/if}
	{#if onsettings}
		<IconButton name="gear" label="Настройки" onclick={() => onsettings()} />
	{/if}
</div>

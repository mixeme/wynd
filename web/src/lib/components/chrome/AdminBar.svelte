<script lang="ts">
	import AdminNav, { type AdminNavItem } from '$ui/admin/AdminNav.svelte';
	import Mark from '$ui/Mark.svelte';

	let {
		active = 'Хранилище',
		server,
		nav = true,
		links = false
	}: {
		active?: AdminNavItem;
		server?: string;
		nav?: boolean;
		links?: boolean;
	} = $props();
</script>

<div class="admbar">
	<Mark />
	{#if nav}
		<AdminNav {active} {links} />
	{/if}
	{#if server}
		<span style="margin-left:auto;font-size:12.5px;color:var(--faint)">{server}</span>
	{/if}
</div>

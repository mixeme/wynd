<script lang="ts">
	import { page } from '$app/state';
	import CirclesSplitLayout from '$lib/layouts/CirclesSplitLayout.svelte';
	import SettingsNav from '$lib/layouts/SettingsNav.svelte';
	import { splitLayout } from '$lib/session/layout.svelte';
	import type { Snippet } from 'svelte';

	// План 76: на широком окне у общих настроек своя левая колонка — пункты
	// настроек, справа выбранный раздел. На телефоне всё как раньше.
	let { children }: { children: Snippet } = $props();

	const path = $derived(page.url.pathname.replace(/\/+$/, '') || '/');
	const split = $derived(splitLayout());
	const empty = $derived(path === '/settings');
</script>

{#if split}
	<CirclesSplitLayout
		{empty}
		emptyTitle="Выберите раздел"
		emptyText="Откройте раздел в списке слева — он появится здесь."
	>
		{#snippet sidebar()}
			<SettingsNav />
		{/snippet}
		{@render children()}
	</CirclesSplitLayout>
{:else}
	{@render children()}
{/if}

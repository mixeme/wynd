<script lang="ts">
	import { onMount } from 'svelte';
	import NewsBlock from '$ui/data/NewsBlock.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { formatSessionDay } from '$lib/format/time';
	import { navigateBack } from '$lib/navigation/up';
	import { NEWS } from '$lib/news/notes';
	import { markNewsSeen } from '$lib/news/seen';

	// Что нового (7.8). Открывается с баннера на списке кругов и из
	// «Настроек»; откуда пришли — в адресе, «Назад» ведёт туда же.

	onMount(() => markNewsSeen());
</script>

<FormLayout shell app title="Что нового" onback={navigateBack}>
	{#each NEWS as entry (entry.version)}
		<NewsBlock version={entry.version} day={formatSessionDay(entry.date)} items={entry.items} />
	{/each}
</FormLayout>

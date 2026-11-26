<script lang="ts">
	import SearchPane from '$lib/layouts/SearchPane.svelte';
	import { wideSource } from '$lib/navigation/wideSource.svelte';
	import { untrack } from 'svelte';

	const sourceUrl = untrack(() => wideSource()?.url);
</script>

<!-- План 76: на широком окне поиск стоит слева, в каркасе /search/+layout;
     правая часть без выбора пустая. На телефоне — как было. -->
<SearchPane {sourceUrl} />

<script lang="ts">
	import type { SpaceNotice } from '$lib/circles/space';

	// Полоса «места нет» наверху экрана (4.25): писать нельзя — что и кто
	// поможет. Внизу ленты та же полоса стоит вместо строки ввода (3.10).
	let { notice }: { notice: SpaceNotice } = $props();
</script>

<div class="write-note-top" role="status">
	{notice.first}<br />
	{#if notice.href}
		<a class="under" href={notice.href}>{notice.second}</a>
	{:else}
		{notice.second}
	{/if}
</div>

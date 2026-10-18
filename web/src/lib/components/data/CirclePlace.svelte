<script lang="ts">
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Meter from '$ui/forms/Meter.svelte';
	import { formatBytes } from '$lib/format/bytes';

	// «Место» круга (6.15, 6.18): шкала и «776 МБ из 1,0 ГБ» — одинаково на
	// «Архиве и очистке» и на «Попросить место».
	let { usedBytes, quotaBytes }: { usedBytes: number; quotaBytes: number } = $props();

	const GB = 1024 * 1024 * 1024;
	// The instance may run without a storage limit: the API then omits quota_bytes.
	const capped = $derived(quotaBytes > 0);
	const full = $derived(capped && usedBytes >= quotaBytes);
</script>

<Label>Место</Label>
<Meter value={capped ? usedBytes / GB : 0} max={Math.max(quotaBytes / GB, 0.001)} />
<Hint class="mt-8">
	{#if capped}
		{formatBytes(usedBytes)} из {formatBytes(quotaBytes)}
	{:else}
		{formatBytes(usedBytes)} · ограничение не задано
	{/if}
	{#if full}· писать в круг больше нельзя{/if}
</Hint>

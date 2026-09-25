<!-- Smoke-test: ShellLayout from docs/screens.html #e2-15 -->
<script lang="ts">
	import CircleRow from '$ui/data/CircleRow.svelte';
	import Icon from '$ui/Icon.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import ShellLayout from '$lib/layouts/ShellLayout.svelte';
</script>

<ShellLayout onsearch={() => {}} onsettings={() => {}}>
	{#snippet fab()}
		<Icon name="plus" style="width:26px;height:26px;stroke-width:1.5" />
	{/snippet}
	<SectionLabel>Закреплённые</SectionLabel>
	<CircleRow
		initial="С"
		name="Семья"
		preview="Аня: были на даче, все живы"
		time="14:02"
		badge={3}
		color="var(--terracotta)"
	/>
	<div class="sep"></div>
	<SectionLabel>Остальные</SectionLabel>
	<CircleRow
		initial="П"
		name="Поход на Алтай"
		preview="читает, не пишет"
		time="вчера"
		color="var(--olive)"
	/>
	<CircleRow
		initial="У"
		name="Универ"
		preview="Петя: нашёл фотки с 3 курса"
		time="пн"
		color="var(--indigo)"
	/>
</ShellLayout>

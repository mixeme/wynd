<!-- Smoke-test: ShellLayout from docs/screens.html #e2-1 -->
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
	<CircleRow
		initial="П"
		name="Поход на Алтай"
		preview="Мышка теперь Мышь"
		time="вчера"
		badge={1}
		color="var(--olive)"
	/>
	<CircleRow
		initial="У"
		name="Универ"
		preview="Петя: нашёл фотки с 3 курса"
		time="пн"
		color="var(--indigo)"
	/>
	<CircleRow
		initial="Д"
		name="Дача"
		preview="Мама: яблони зацвели"
		time="5 авг"
		color="var(--ochre)"
	/>
	<CircleRow
		initial="С"
		name="Соседи"
		preview="Игорь присоединился"
		time="28 июл"
		color="var(--slate)"
	/>
	<CircleRow
		initial="Р"
		name="Ремонт"
		preview="Славик: плитку привезут в среду"
		time="24 июл"
		color="var(--coffee)"
	/>
</ShellLayout>

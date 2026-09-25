<!-- Smoke-test: data components from docs/screens.html #e5-1 -->
<script lang="ts">
	import CircleBar from '$ui/chrome/CircleBar.svelte';
	import DayCard from '$ui/data/DayCard.svelte';
	import DayGrid from '$ui/data/DayGrid.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import MonthLabel from '$ui/data/MonthLabel.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import StatusBar from '$ui/chrome/StatusBar.svelte';
</script>

<PhoneFrame color="terracotta">
	<StatusBar />
	<CircleBar title="Семья" identity="Мышь" avatar="М" active="Дни" />
	<Hint style="margin-top:12px">112 дней из 340 записей</Hint>
	<MonthLabel>Август 2026</MonthLabel>
	<DayGrid>
		<DayCard
			title="Дача, яблони"
			subtitle="12 августа · 4 записи"
			cover="p1"
			photoCount={12}
		/>
		<DayCard title="11 августа" subtitle="2 записи" cover="p5" photoCount={3} />
		<DayCard title="Разговор" subtitle="9 августа · 1 запись" />
		<DayCard title="6 августа" subtitle="1 запись" cover="p6" />
	</DayGrid>
	<MonthLabel>Июль 2026</MonthLabel>
	<DayGrid>
		<DayCard title="Река" subtitle="12 июля · 1 запись" cover="p2" photoCount={7} />
		<DayCard title="Проводы" subtitle="4 июля · 6 записей" cover="p3" />
	</DayGrid>
</PhoneFrame>

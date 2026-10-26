<!-- Smoke-test: forms components from docs/screens.html #e6-17 -->
<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import CircleBar from '$ui/chrome/CircleBar.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import StatusBar from '$ui/chrome/StatusBar.svelte';
	import VolumeChart from '$ui/forms/VolumeChart.svelte';
	import CirclePlace from '$ui/data/CirclePlace.svelte';
	import { volumeBarEdgeX } from '$lib/circles/volumeChart';

	// Тёмная тема (владелец, 2026-10-25: «Столбик выглядит странно. Он
	// двухцветный»): один день, дата — его начало; и полгода, выбран апрель.
	const oneDay = [{ period: '2026-10-25', bytes: 4000, cumulative_bytes: 4000 }];
	const months = ['2025-11', '2025-12', '2026-01', '2026-02', '2026-03', '2026-04', '2026-05'].map(
		(period, i) => ({
			period,
			bytes: ((i % 3) + 1) * 1000,
			cumulative_bytes: 0
		})
	);
</script>

<!-- С 0.33.1 (C43) — отдельный экран за строкой на 6.15: шкала места и
     «Попросить место» остались там. -->
<PhoneFrame color="terracotta">
	<StatusBar />
	<CircleBar title="Заархивировать и удалить старое" tabs={false} />
	<Label>Сколько освободится · по месяцам</Label>
	<VolumeChart />
	<Label style="margin-top:14px">Архивировать всё до</Label>
	<FieldDisplay value="1 июня 2025" active />
	<Hint>Освободится 6,2 ГБ из 10. В круге останется 214 записей из 340.</Hint>
	<Button variant="colored" onclick={() => {}}>Дальше: сроки</Button>
</PhoneFrame>

<PhoneFrame color="terracotta" dark>
	<StatusBar />
	<CircleBar title="Заархивировать и удалить старое" tabs={false} />
	<!-- «Место» почти пустого круга: подложка шкалы тёмная, а не светлая полоса. -->
	<CirclePlace usedBytes={446} quotaBytes={2 * 1024 * 1024 * 1024} />
	<Label>Сколько освободится · по дням</Label>
	<VolumeChart
		volume={oneDay}
		cutoffLabel="25 октября"
		cutoffX={volumeBarEdgeX(0, 1)}
		oncutoff={() => {}}
	/>
	<Label style="margin-top:14px">Архивировать всё до</Label>
	<FieldDisplay value="25.10.2026" active />
	<Hint>До этой даты записей нет — выберите дату позже.</Hint>
	<Label style="margin-top:20px">Сколько освободится · по месяцам</Label>
	<VolumeChart
		volume={months}
		cutoffLabel="1 апреля"
		cutoffX={volumeBarEdgeX(5, months.length)}
		oncutoff={() => {}}
	/>
</PhoneFrame>

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

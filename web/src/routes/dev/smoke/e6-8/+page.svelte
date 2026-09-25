<!-- Smoke-test: forms components from docs/screens.html #e6-8 -->
<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import CircleBar from '$ui/chrome/CircleBar.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Meter from '$ui/forms/Meter.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import StatusBar from '$ui/chrome/StatusBar.svelte';
	import VolumeChart from '$ui/forms/VolumeChart.svelte';
</script>

<PhoneFrame color="terracotta">
	<StatusBar />
	<CircleBar title="Архив и очистка" tabs={false} />
	<Label>Место</Label>
	<Meter value={10} max={10} />
	<Hint style="margin-top:8px"
		>10 ГБ из 10 ГБ · фотографии больше не загружаются, текст пишется</Hint
	>
	<SettingsRow
		title="Попросить у администратора"
		subtitle="шаг необязательный — можно сразу к отсечке"
		style="margin-top:10px;border-top:1px solid var(--line);border-bottom:1px solid var(--line)"
	/>
	<Label style="margin-top:18px">Сколько освободит отсечка</Label>
	<VolumeChart />
	<Label style="margin-top:14px">Архивировать всё до</Label>
	<FieldDisplay value="1 июня 2025" active />
	<Hint>Освободится 6,2 ГБ из 10. В круге останется 214 записей из 340.</Hint>
	<Button variant="colored" onclick={() => {}}>Дальше: сроки</Button>
</PhoneFrame>

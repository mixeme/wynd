<!-- Smoke-test: forms components from docs/screens.html #e6-2 (низ) -->
<script lang="ts">
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import CircleBar from '$ui/chrome/CircleBar.svelte';
	import DangerZone from '$ui/forms/DangerZone.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import MemberRow from '$ui/data/MemberRow.svelte';
	import Meter from '$ui/forms/Meter.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import StatusBar from '$ui/chrome/StatusBar.svelte';
</script>

<PhoneFrame color="terracotta">
	<StatusBar />
	<CircleBar title="Настройки круга" tabs={false} />
	<div style="height:34px;overflow:hidden">
		<ChipGroup style="margin-top:-14px">
			<Chip selected>Одноразовые</Chip>
			<Chip>Многоразовые</Chip>
		</ChipGroup>
	</div>
	<SettingsRow title="Кто ты в этом круге" value="Мышь" />
	<Label style="margin-top:20px">Место</Label>
	<Meter value={8.4} max={10} />
	<Hint style="margin-top:8px">8,4 ГБ из 10 ГБ в «Доме Ани» · квоту задал администратор</Hint>
	<SettingsRow
		title="Архив и очистка"
		subtitle="освободить место, скачать архив"
		style="margin-top:8px"
	/>
	<Label style="margin-top:20px">Участники · 22</Label>
	<MemberRow
		initial="К"
		name="Кот"
		subtitle="владелец"
		color="#62452F"
		style="padding-top:2px"
	/>
	<MemberRow
		initial="А"
		name="Аня"
		subtitle="может менять настройки"
		color="#58673A"
	/>
	<MemberRow initial="М" name="Мышь" subtitle="это вы" />
	<SettingsRow title="ещё 19" link />
	<DangerZone
		items={['Передать владение', 'Покинуть круг', 'Удалить круг и все записи']}
	/>
</PhoneFrame>

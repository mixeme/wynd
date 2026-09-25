<!-- Smoke-test: data components from docs/screens.html #e6-3 -->
<script lang="ts">
	import Button from '$ui/forms/Button.svelte';
	import CircleBar from '$ui/chrome/CircleBar.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import MemberRow from '$ui/data/MemberRow.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import StatusBar from '$ui/chrome/StatusBar.svelte';
</script>

<PhoneFrame color="terracotta">
	<StatusBar />
	<CircleBar title="Участники" identity="22" tabs={false} />
	<SectionLabel style="margin-top:0">В круге · 20</SectionLabel>
	<MemberRow
		initial="К"
		name="Кот"
		subtitle="владелец · с 12 марта 2024"
		color="#62452F"
		style="padding-top:2px"
	/>
	<MemberRow
		initial="А"
		name="Аня"
		subtitle="может менять настройки"
		color="#58673A"
		menu
		onmenu={() => {}}
	/>
	<MemberRow initial="М" name="Мышь" subtitle="это вы · с 5 августа" />
	<MemberRow initial="М" name="Мама" color="#3C4D83" menu onmenu={() => {}} />
	<MemberRow initial="П" name="Петя" color="#357077" menu onmenu={() => {}} />
	<SettingsRow title="ещё 15" link />
	<SectionLabel style="margin-top:18px">Вышли · 2</SectionLabel>
	<MemberRow
		initial="Д"
		name="Дед"
		subtitle="читает, не пишет · до 2 июня"
		color="#7A4265"
		faded
		style="padding-top:2px"
	/>
	<MemberRow
		initial="Л"
		name="Люся"
		subtitle="без доступа · до 14 января"
		color="#3D494F"
		faded
	/>
	<Hint style="margin-top:16px">
		Записи вышедших остаются в круге и подписаны тем именем, что было на момент написания.
	</Hint>
	<Button variant="ghost" onclick={() => {}}>Пригласить</Button>
</PhoneFrame>

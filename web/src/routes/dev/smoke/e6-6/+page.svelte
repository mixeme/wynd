<!-- Smoke-test: SettingsRow + Switch from docs/screens.html #e6-6 -->
<script lang="ts">
	import CircleBar from '$ui/chrome/CircleBar.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import StatusBar from '$ui/chrome/StatusBar.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Switch from '$ui/forms/Switch.svelte';

	let posts = $state(true);
	let myComments = $state(true);
	let allComments = $state(false);
	let reactions = $state(false);
	let events = $state(false);
</script>

<PhoneFrame color="terracotta">
	<StatusBar />
	<CircleBar title="Уведомления круга" tabs={false} />
	<Label>Присылать</Label>
	<SettingsRow title="Новые записи" style="padding-top:2px">
		{#snippet control()}
			<Switch bind:checked={posts} />
		{/snippet}
	</SettingsRow>
	<SettingsRow title="Комментарии к моим записям">
		{#snippet control()}
			<Switch bind:checked={myComments} />
		{/snippet}
	</SettingsRow>
	<SettingsRow title="Все комментарии">
		{#snippet control()}
			<Switch bind:checked={allComments} />
		{/snippet}
	</SettingsRow>
	<SettingsRow title="Реакции">
		{#snippet control()}
			<Switch bind:checked={reactions} />
		{/snippet}
	</SettingsRow>
	<SettingsRow title="Упоминания" subtitle="всегда">
		{#snippet control()}
			<Switch checked={true} disabled />
		{/snippet}
	</SettingsRow>
	<SettingsRow title="События круга">
		{#snippet control()}
			<Switch bind:checked={events} />
		{/snippet}
	</SettingsRow>
	<Hint style="margin-top:14px"
		>Пуш не несёт текста — только круг, тип события и число. Что именно написано, устройство
		узнаёт после синхронизации и дорисовывает само.</Hint
	>
</PhoneFrame>

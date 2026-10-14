<!-- Smoke-test: FormLayout from docs/screens.html #e7-30 -->
<script lang="ts">
	import DangerZone from '$ui/forms/DangerZone.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
</script>

<FormLayout shell title="Уйти с сервера насовсем" onback={() => {}}>
	<ServerRow name="Дом Ани" subtitle="home.example.org" card top={10} />
	<SettingsRow title="anya@example.com" subtitle="5 кругов" chevron={false} />
	<SectionLabel class="mt-18">Сначала — ваши круги</SectionLabel>
	<Hint class="mt-0 ink">
		Вы владелец двух кругов. Без владельца круг остаться не может: передайте его кому-то из
		участников или удалите.
	</Hint>
	<SettingsRow
		class="mt-8"
		title="Семья"
		subtitle="22 участника · передать владение"
		divided="top"
		onclick={() => {}}
	/>
	<SettingsRow
		title="Заметки"
		subtitle="кроме вас никого · удалить круг"
		divided="bottom"
		onclick={() => {}}
	/>
	<DangerZone class="mt-22" disabled items={['Уйти с «Дома Ани» насовсем']} />
</FormLayout>

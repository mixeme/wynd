<!-- Smoke-test: FormLayout from docs/screens.html #e7-29 (и #e7-32 — переключатель включён) -->
<script lang="ts">
	import DangerZone from '$ui/forms/DangerZone.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';

	let erase = $state(false);
</script>

<FormLayout shell title="Уйти с сервера насовсем" onback={() => {}}>
	<ServerRow name="Дом Ани" subtitle="home.example.org" card gdpr top={10} />
	<SettingsRow title="anya@example.com" subtitle="5 кругов" chevron={false} />
	<SectionLabel class="mt-18">Исчезнет</SectionLabel>
	<Hint class="mt-0 ink">
		Вход на home.example.org на всех ваших устройствах, уведомления отсюда. Вернуться с этой
		почтой можно — как новый человек.
	</Hint>
	<SectionLabel class="mt-18">Останется в кругах</SectionLabel>
	<Hint class="mt-0 ink">
		Всё, что вы написали и выложили, — под вашими именами, как у тех, кто ушёл из круга.
	</Hint>
	<SettingsRow
		class="mt-14"
		title="Стереть моё имя и фото во всех кругах"
		subtitle="записи останутся без имени"
		divided="both"
	>
		{#snippet control()}
			<Switch bind:checked={erase} label="Стереть моё имя и фото во всех кругах" />
		{/snippet}
	</SettingsRow>
	<Hint>Ваши круги на других серверах останутся как есть.</Hint>
	<DangerZone class="mt-18" items={['Уйти с «Дома Ани» насовсем']} onitem={() => {}} />
</FormLayout>

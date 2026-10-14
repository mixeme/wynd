<!-- Smoke-test: ConfirmDialog from docs/screens.html #e7-31 -->
<script lang="ts">
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import ConfirmDialog from '$ui/overlays/ConfirmDialog.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';

	let name = $state('Дом Ани');
</script>

<FormLayout shell title="Уйти с сервера насовсем" onback={() => {}}>
	<ServerRow name="Дом Ани" subtitle="home.example.org" card gdpr top={10} />
</FormLayout>
<ConfirmDialog
	title="Уйти с «Дома Ани» насовсем?"
	confirmLabel="Уйти"
	confirmDisabled={name.trim() !== 'Дом Ани'}
	onconfirm={() => {}}
	oncancel={() => {}}
>
	<Hint class="mb-14">
		Вы выйдете со всех устройств. Вернуться с этой почтой можно, но как новый человек: в круги —
		по новому приглашению, и прежнее в них будет не видно. Ваши записи останутся под вашими
		именами.
	</Hint>
	<Input active placeholder="Дом Ани" bind:value={name} />
</ConfirmDialog>

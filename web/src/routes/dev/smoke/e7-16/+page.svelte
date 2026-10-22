<!-- Smoke-test: ConfirmDialog from docs/screens.html #e7-16 -->
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
	title="Уйти с сервера «Дом Ани» насовсем?"
	confirmLabel="Уйти"
	confirmDisabled={name.trim() !== 'Дом Ани'}
	onconfirm={() => {}}
	oncancel={() => {}}
>
	<Hint class="mb-14">Впишите имя сервера, чтобы уйти.</Hint>
	<Input active placeholder="Дом Ани" bind:value={name} />
</ConfirmDialog>

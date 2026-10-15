<script lang="ts">
	import { page } from '$app/stores';
	import Button from '$ui/forms/Button.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { copyText } from '$lib/clipboard';

	// Ссылку переноса (план 53, 6.25) открыли в браузере. Её дело — новый
	// сервер, а не страница: говорим, куда её вставить (кадра нет).
	const link = $derived($page.url.href);
	let copied = $state(false);

	async function copy() {
		await copyText(link);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}
</script>

<FormLayout shell app title="Ссылка переноса" onback={() => history.back()}>
	<FieldDisplay mono value={link} />
	<Button class="mt-12" variant="ghost" onclick={() => void copy()}>
		{copied ? 'Скопировано' : 'Скопировать'}
	</Button>
	<Hint class="mt-10">
		Это ссылка переноса круга. Открывать её не нужно — её вставляют на новом сервере: «Настройки» →
		«Серверы» → «Принять круг» → «Перенести с другого сервера».
	</Hint>
</FormLayout>

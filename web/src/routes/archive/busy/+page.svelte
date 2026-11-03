<script lang="ts">
	import { page } from '$app/stores';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import Label from '$ui/forms/Label.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import { archiveBusyMessage, archiveRetrySeconds } from '$lib/circles/archive-download.svelte';

	// 6.23б: the valid one-time token was already checked by the API. This page
	// only explains a busy response; it has no account data or session gate.
	let remaining = $state(30);
	$effect(() => {
		const seconds = archiveRetrySeconds($page.url.searchParams.get('retry_after'));
		remaining = seconds;
		const end = Date.now() + seconds * 1000;
		const timer = setInterval(() => {
			remaining = Math.max(0, Math.ceil((end - Date.now()) / 1000));
			if (remaining === 0) clearInterval(timer);
		}, 1000);
		return () => clearInterval(timer);
	});
</script>

<FormLayout shell app title="Архив круга">
	<Label>Скачивание</Label>
	<SettingsRow icon="clock" title="Архив уже скачивается" chevron={false} divided="both" />
	<Hint class="mt-16">{archiveBusyMessage(remaining, true)}</Hint>
	<Hint>Эту страницу можно закрыть. Закрытие не отменит ранее начатое скачивание.</Hint>
</FormLayout>

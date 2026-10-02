<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import { goto } from '$app/navigation';
	import Hint from '$ui/forms/Hint.svelte';
	import QrScanner from '$ui/forms/QrScanner.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { inviteTarget } from '$lib/auth/links';
	import { page } from '$app/stores';

	// 2.17: QR-код приглашения показывают на чужом экране (2.7, 9.2) — значит,
	// считывать его логично здесь же, камерой.
	let error = $state('');
	let notice = $state('');

	function onread(text: string): boolean {
		const target = inviteTarget(text);
		if ('path' in target) {
			goto(target.path, { replaceState: true });
			return true;
		}
		// Чужой код — сказать и ждать следующего: камера остаётся включённой.
		notice = target.error;
		return false;
	}
</script>

<FormLayout shell app title="Сканер" onback={() => goUp($page.url.searchParams.get('from') === 'join' ? '/join' : '/invite')}>
	<QrScanner {onread} onerror={(message) => (error = message)} />
	{#if error}
		<Hint>{error}</Hint>
	{:else if notice}
		<Hint>{notice}</Hint>
	{:else}
		<Hint>Наведите камеру на QR-код приглашения — он откроется сам.</Hint>
	{/if}
</FormLayout>

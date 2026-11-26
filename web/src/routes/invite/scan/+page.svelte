<script lang="ts">
	import { navigateBack } from '$lib/navigation/up';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import Hint from '$ui/forms/Hint.svelte';
	import QrScanner from '$ui/forms/QrScanner.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { inviteTarget, loginLinkTarget } from '$lib/auth/links';

	// 2.8: QR-код приглашения показывают на чужом экране (2.17, 9.2) — значит,
	// считывать его логично здесь же, камерой.
	let error = $state('');
	let notice = $state('');
	// Пришли с экрана входа (план 72): ждём QR подключения устройства, а не приглашения.
	const fromLogin = $derived(page.url.searchParams.get('from') === 'login');
	// Пришли с «Войти на сервер»: QR подключения — это добавление сервера
	// (адрес и почта в форму), интерфейс не меняется (план 72).
	const fromJoin = $derived(page.url.searchParams.get('from') === 'join');

	function onread(text: string): boolean {
		const login = loginLinkTarget(text);
		if (login && fromJoin) {
			goto(
				`/join?from=servers&server=${encodeURIComponent(login.origin)}&email=${encodeURIComponent(login.email)}`,
				{ replaceState: true }
			);
			return true;
		}
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

<FormLayout shell app title="Сканер" onback={navigateBack}>
	<QrScanner {onread} onerror={(message) => (error = message)} />
	{#if error}
		<Hint>{error}</Hint>
	{:else if notice}
		<Hint>{notice}</Hint>
	{:else if fromJoin}
		<!-- Ссылка подключения устройства (план 72): добавляет сервер, не меняя интерфейс. -->
		<Hint>Наведите камеру на QR-код подключения — сервер добавится сам.</Hint>
	{:else if fromLogin}
		<!-- Ссылка подключения устройства (план 72): адрес сервера и почта. -->
		<Hint>Наведите камеру на QR-код подключения — вход откроется сам.</Hint>
	{:else}
		<Hint>Наведите камеру на QR-код приглашения — он откроется сам.</Hint>
	{/if}
</FormLayout>

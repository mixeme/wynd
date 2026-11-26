<script lang="ts">
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import { navigateBack } from '$lib/navigation/up';
	import { displayHost } from '$lib/auth/origin';
	import { pageOrigin } from '$lib/desktop/bridge';
	import { loadSessions } from '$lib/session/session.svelte';
	import { loadServerCard } from '$lib/instance/serverCard';
	import { copyText } from '$lib/clipboard';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import QrCode from '$ui/data/QrCode.svelte';
	import InviteLinkCard from '$ui/data/InviteLinkCard.svelte';

	// У сервера на своём адресе origin — пустая строка: не проверять как истину.
	let origin = $state($page.url.searchParams.get('origin') ?? '');

	let email = $state('');
	let serverName = $state('');
	let loading = $state(true);
	let copied = $state(false);

	// Ссылка входа: адрес сервера и почта (план 72). Токена внутри нет —
	// показ QR доступа не даёт, код из письма подтверждает владельца почты.
	const link = $derived(`${origin || pageOrigin()}/?email=${encodeURIComponent(email)}`);

	async function copy() {
		await copyText(link);
		copied = true;
		setTimeout(() => (copied = false), 2000);
	}

	async function share() {
		const nav = navigator as Navigator & { share?: (data: ShareData) => Promise<void> };
		if (nav.share) {
			await nav.share({ title: serverName || 'Wynd', url: link });
			return;
		}
		await copy();
	}

	onMount(() => {
		void (async () => {
			const sessions = await loadSessions();
			email = sessions.find((s) => s.origin === origin)?.email ?? '';
			const card = await loadServerCard(origin).catch(() => undefined);
			serverName = card?.info?.name ?? '';
			loading = false;
		})();
	});
</script>

<FormLayout shell app title="Подключить устройство" onback={navigateBack}>
	{#if loading}
		<Loading />
	{:else}
		<Hint class="mt-10">
			Отсканируйте камерой на новом устройстве: откроется вход на «{serverName ||
				displayHost(origin)}», почта уже подставлена — останется «Получить код» и ввести его из
			письма. Ссылкой можно поделиться.
		</Hint>
		<QrCode value={email ? link : ''} />
		<InviteLinkCard url={link} {copied} onshare={() => void share()} oncopy={() => void copy()} />
	{/if}
</FormLayout>

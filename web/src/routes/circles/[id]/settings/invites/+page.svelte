<script lang="ts">
	import { copyText } from '$lib/clipboard';
	import { goto } from '$app/navigation';
	import { getContext, onMount } from 'svelte';
	import QRCode from 'qrcode';
	import Button from '$ui/forms/Button.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import Label from '$ui/forms/Label.svelte';
	import OverlayLayout from '$lib/layouts/OverlayLayout.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		inviteRegistrySubtitle,
		inviteRegistryTitle
	} from '$lib/circles/invite-registry';
	import {
		fetchCircleInvites,
		fetchCircleSettings,
		revokeCircleInvite,
		type CircleInvite
	} from '$lib/circles/settings';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let liveInvites = $state<CircleInvite[]>([]);
	let multiInvitesAllowed = $state(false);
	let error = $state('');
	let loading = $state(true);
	let copiedInviteId = $state('');
	// Ссылка, открытая нажатием на строку: QR тому, кто рядом, ссылка —
	// кому отправить (6.21). Действия ссылки живут здесь же.
	let opened = $state<CircleInvite | null>(null);
	let openedQr = $state('');
	let shared = $state(false);

	async function openInvite(inv: CircleInvite) {
		opened = inv;
		shared = false;
		openedQr = await QRCode.toString(inviteUrlFor(inv.token), { type: 'svg', margin: 0, width: 168 });
	}

	function closeInvite() {
		opened = null;
		openedQr = '';
	}

	async function shareInvite(inv: CircleInvite) {
		const url = inviteUrlFor(inv.token);
		if (navigator.share) {
			try {
				await navigator.share({ url, title: `Приглашение в «${circle.name}»` });
				shared = true;
				return;
			} catch (err) {
				if (err instanceof Error && err.name === 'AbortError') return;
			}
		}
		await copyInvite(inv);
	}

	const hasLiveMulti = $derived(liveInvites.some((inv) => inv.kind === 'multi'));

	function goBack() {
		goto(`/circles/${circle.circleId}/settings`);
	}

	function goInvite() {
		goto(`/circles/${circle.circleId}/settings/invite`);
	}

	// Ссылка ведёт на сервер круга, а не на хост, с которого открыт клиент:
	// иначе токен круга с сервера B уходил в логи сервера A (аудит 2026-09-22).
	function inviteUrlFor(token: string): string {
		const base = circle.origin || (typeof window !== 'undefined' ? window.location.origin : '');
		return `${base}/invite/${token}`;
	}

	async function loadLiveInvites() {
		liveInvites = await fetchCircleInvites(circle.origin, circle.circleId);
	}

	async function copyInvite(inv: CircleInvite) {
		await copyText(inviteUrlFor(inv.token));
		copiedInviteId = inv.id;
	}

	async function revokeInvite(inv: CircleInvite) {
		try {
			await revokeCircleInvite(circle.origin, circle.circleId, inv.id);
			closeInvite();
			await loadLiveInvites();
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	onMount(async () => {
		try {
			const settings = await fetchCircleSettings(circle.origin, circle.circleId);
			multiInvitesAllowed = settings.invite_kind_default === 'multi';
			await loadLiveInvites();
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	});
</script>

<FormLayout app color={circle.color} title="Живые ссылки" onback={goBack}>
	{#if loading}
		<Loading />
	{:else if error}
		<Hint class="gutter">{error}</Hint>
	{:else if liveInvites.length === 0}
		<Hint class="gutter">Живых ссылок нет.</Hint>
		<Button variant="ghost" onclick={goInvite}>Пригласить</Button>
	{:else}
		{#each liveInvites as inv (inv.id)}
			<SettingsRow
				title={inviteRegistryTitle(inv)}
				subtitle={inviteRegistrySubtitle(inv)}
				onclick={() => void openInvite(inv)}
			/>
		{/each}
		{#if !multiInvitesAllowed && hasLiveMulti}
			<Hint class="mt-8"
				>Эту многоразовую успели создать раньше. Новую уже нельзя.</Hint
			>
		{/if}
	{/if}
</FormLayout>

{#if opened}
	{@const inv = opened}
	<OverlayLayout label={inviteRegistryTitle(inv)} ondismiss={closeInvite}>
		<Label class="mt-2">{inviteRegistryTitle(inv)}</Label>
		{#if openedQr}
			<div class="qr">{@html openedQr}</div>
		{/if}
		<FieldDisplay mono value={inviteUrlFor(inv.token)} class="invite-url" />
		<div class="rowin ask">
			<Button variant="colored" onclick={() => void shareInvite(inv)}>
				{shared ? 'Отправлено' : 'Поделиться'}
			</Button>
			<Button variant="ghost" onclick={() => void copyInvite(inv)}>
				{copiedInviteId === inv.id ? 'Скопировано' : 'Скопировать'}
			</Button>
		</div>
		<div class="hint ctr mt-16">
			<TextButton onclick={() => void revokeInvite(inv)}>Отозвать ссылку</TextButton>
		</div>
	</OverlayLayout>
{/if}

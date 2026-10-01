<script lang="ts">
	import EmptyState from '$ui/data/EmptyState.svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import PlainLayout from '$lib/layouts/PlainLayout.svelte';
	import { authErrorHint, fetchInstance, sendAuthCode } from '$lib/auth/auth';
	import {
		fetchInvitePeek,
		isDeadInviteError,
		isCircleInvitePeek,
		serverInviteSubtitle,
		type InvitePeek
	} from '$lib/auth/invites';
	import { displayHost } from '$lib/auth/origin';
	import { INVALID_EMAIL_HINT, isValidParticipantEmail } from '$lib/auth/email';
	import { loadPendingAuth, savePendingAuth } from '$lib/auth/pending';

	let { data } = $props();
	const token = $derived(data.token);

	let email = $state('');
	let peek = $state<InvitePeek | undefined>();
	let loading = $state(false);
	let error = $state('');
	// Ссылка мертва — вместо формы экран с причиной (см. isDeadInviteError).
	let dead = $state(false);

	onMount(async () => {
		const pending = loadPendingAuth();
		if (pending?.inviteToken === token) {
			email = pending.email;
		}
		try {
			const next = await fetchInvitePeek('', token);
			if (isCircleInvitePeek(next)) {
				goto(`/invite/${token}`);
				return;
			}
			peek = next;
		} catch (err) {
			if (isDeadInviteError(err)) dead = true;
			else error = 'Не удалось проверить приглашение — сервер не отвечает. Попробуйте позже.';
		}
	});

	async function onSubmit() {
		error = '';
		const trimmed = email.trim();
		if (!trimmed) {
			error = 'Введите почту';
			return;
		}
		if (!isValidParticipantEmail(trimmed)) {
			error = INVALID_EMAIL_HINT;
			return;
		}
		loading = true;
		try {
			const info = await fetchInstance('');
			const pending = {
				origin: '',
				email: trimmed,
				flow: 'invite' as const,
				inviteToken: token,
				inviteName: '',
				circleInvite: false,
				instanceName: info.name,
				codeDelivery: info.code_delivery ?? 'mail'
			};
			await sendAuthCode(pending);
			savePendingAuth({ ...pending, codeSentAt: Date.now() });
			goto('/auth/code');
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}

	const serverName = $derived(peek?.server_name || '…');
	const serverSubtitle = $derived(
		peek ? serverInviteSubtitle(peek, displayHost('')) : '…'
	);
</script>

{#if dead}
	<PlainLayout shell app>
		<EmptyState title="Приглашение не действует">
			Ссылку отозвали, она истекла или по ней уже вошли. Попросите новую у того, кто держит сервер.
			{#snippet actions()}
				<Button onclick={() => goto('/')}>На главную</Button>
			{/snippet}
		</EmptyState>
	</PlainLayout>
{:else}
<PlainLayout shell app>
	<ScreenTitle class="mt-24">Вас позвали на сервер</ScreenTitle>
	<ServerRow name={serverName} subtitle={serverSubtitle} card />
	<Label class="mt-16">Почта</Label>
	<Input active type="email" autocomplete="email" bind:value={email} />
	<Hint>Пришлём код для входа. Пароля нет.</Hint>
	<Button {loading} disabled={!peek} onclick={onSubmit}>Получить код</Button>
	<Hint>
		Сервер хранит данные незашифрованными. Присоединение к этому серверу означает, что вы
		доверяете его администратору.
	</Hint>
	<Hint class="mt-26" centered>
		Приглашение на сервер не ведёт ни в один круг:<br />заведёте свой или подождёте, пока позовут.
	</Hint>
	{#if error}
		<Hint class="mt-12">{error}</Hint>
	{/if}
</PlainLayout>
{/if}

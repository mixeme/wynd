<script lang="ts">
	import EmptyState from '$ui/data/EmptyState.svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import Button from '$ui/forms/Button.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import InviteCard from '$ui/forms/InviteCard.svelte';
	import MemberRow from '$ui/data/MemberRow.svelte';
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import PlainLayout from '$lib/layouts/PlainLayout.svelte';
	import {
		authErrorHint,
		fetchInstance,
		sendAuthCode
	} from '$lib/auth/auth';
	import {
		claimInvite,
		fetchInvitePeek,
		isDeadInviteError,
		inviteCardPreview,
		isCircleInvitePeek,
		memberAvatarColor,
		memberSubtitle,
		type InvitePeek
	} from '$lib/auth/invites';
	import { rememberCircleOrigin } from '$lib/circles/origin';
	import { getSession } from '$lib/idb/db';
	import { ApiError, isPaymentRequired } from '$lib/api/client';
	import { dropParticipantSession } from '$lib/session/session.svelte';
	import { displayHost } from '$lib/auth/origin';
	import { circleInitial } from '$lib/circles/meta';
	import { INVALID_EMAIL_HINT, isValidParticipantEmail } from '$lib/auth/email';
	import { loadPendingAuth, savePendingAuth } from '$lib/auth/pending';

	let { data } = $props();
	const token = $derived(data.token);

	const showMembers = $derived($page.url.searchParams.get('members') === '1');

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
			if (!isCircleInvitePeek(next)) {
				goto(`/join/${token}`);
				return;
			}
			peek = next;
		} catch (err) {
			if (isDeadInviteError(err)) dead = true;
			else error = 'Не удалось проверить приглашение — сервер не отвечает. Попробуйте позже.';
			return;
		}
		const session = await getSession('');
		if (!session) return;
		try {
			const claim = await claimInvite('', token);
			rememberCircleOrigin(claim.circle_id, '');
			goto(
				claim.already_member ? `/circles/${claim.circle_id}` : `/circles/${claim.circle_id}/join`
			);
		} catch (err) {
			// Сессия на устройстве устарела — обычный вход по почте, без ошибки.
			if (err instanceof ApiError && err.status === 401) {
				await dropParticipantSession('');
				return;
			}
			// Личное приглашение на другую почту: вошли не той учёткой.
			if (err instanceof ApiError && err.status === 403 && !isPaymentRequired(err)) {
				error = 'Это приглашение для другой почты. Введите её ниже.';
				return;
			}
			error = authErrorHint(err);
		}
	});

	async function onSendCode() {
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
				circleInvite: true,
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

	function openMembers() {
		goto(`/invite/${token}?members=1`);
	}

	function closeMembers() {
		goto(`/invite/${token}`);
	}
</script>

{#if dead}
	<PlainLayout shell app>
		<EmptyState title="Приглашение не действует">
			Ссылку отозвали, она истекла, по ней уже вошли или круг удалён. Попросите новую у того, кто вас звал.
			{#snippet actions()}
				<Button onclick={() => goto('/')}>На главную</Button>
			{/snippet}
		</EmptyState>
	</PlainLayout>
{:else if showMembers && peek && isCircleInvitePeek(peek)}
	<FormLayout
		app
		shell
		title="Кто уже здесь"
		right={String(peek.member_count)}
		onback={closeMembers}
	>
		{#each peek.members as member, i (i)}
			<MemberRow
				initial={circleInitial(member.name)}
				name={member.name}
				subtitle={memberSubtitle(member)}
				color={memberAvatarColor(i)}
			/>
		{/each}
	</FormLayout>
{:else}
	<PlainLayout app>
		<ScreenTitle class="mt-24">Вас пригласили</ScreenTitle>
		{#if peek && isCircleInvitePeek(peek)}
			<InviteCard
				initial={circleInitial(peek.circle_name)}
				name={peek.circle_name}
				preview={inviteCardPreview(peek)}
			/>
			<Label>Сервер</Label>
			<FieldDisplay>
				<div class="bold">{peek.server_name}</div>
				<div class="note">{peek.host || displayHost('')}</div>
			</FieldDisplay>
		{:else}
			<InviteCard initial="…" name="…" preview="Загрузка…" />
		{/if}

		<Label>Почта</Label>
		<Input active type="email" autocomplete="email" bind:value={email} />
		<Hint>
			Пришлём код для входа. Пароля нет: почта понадобится, только чтобы вернуться на
			другом устройстве.
		</Hint>
		<Button {loading} disabled={!peek} onclick={onSendCode}>Получить код</Button>

		{#if error}
			<Hint class="mt-12">{error}</Hint>
		{/if}

		<Hint class="mt-26" centered>
			Вход создаётся на этом сервере.<br />Круги с других серверов добавляются позже.
		</Hint>
	</PlainLayout>
{/if}

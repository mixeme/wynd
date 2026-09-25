<script lang="ts">
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
		fetchInvitePeek,
		inviteCardPreview,
		isCircleInvitePeek,
		memberAvatarColor,
		memberSubtitle,
		type InvitePeek
	} from '$lib/auth/invites';
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
		} catch {
			error = 'Приглашение недействительно или истекло';
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

{#if showMembers && peek && isCircleInvitePeek(peek)}
	<FormLayout
		app
		shell
		title="Кто уже здесь"
		right={String(peek.member_count)}
		onback={closeMembers}
	>
		{#each peek.members as member, i (member.name)}
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
		<ScreenTitle style="margin-top:24px">Вас пригласили</ScreenTitle>
		{#if peek && isCircleInvitePeek(peek)}
			<InviteCard
				initial={circleInitial(peek.circle_name)}
				name={peek.circle_name}
				preview={inviteCardPreview(peek)}
			/>
			<Label>Сервер</Label>
			<FieldDisplay>
				<div style="font-weight:600">{peek.server_name}</div>
				<div style="font-size:12.5px;color:var(--muted)">{peek.host || displayHost('')}</div>
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
			<Hint style="margin-top:12px">{error}</Hint>
		{/if}

		<Hint centered style="margin-top:26px">
			Вход создаётся на этом сервере.<br />Круги с других серверов добавляются позже.
		</Hint>
	</PlainLayout>
{/if}

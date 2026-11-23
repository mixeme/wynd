<script lang="ts">
	import PeekMemberList from '$ui/data/PeekMemberList.svelte';
	import EmptyState from '$ui/data/EmptyState.svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import Button from '$ui/forms/Button.svelte';
	import ServerRow from '$ui/data/ServerRow.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import Label from '$ui/forms/Label.svelte';
	import InviteCard from '$ui/forms/InviteCard.svelte';
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import PlainLayout from '$lib/layouts/PlainLayout.svelte';
	import { authErrorHint, fetchInstance, sendAuthCode } from '$lib/auth/auth';
	import {
		claimInvite,
		claimRoute,
		fetchInvitePeek,
		isDeadInviteError,
		inviteCardPreview,
		isCircleInvitePeek,
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
	import { desktopBridge } from '$lib/desktop/bridge';

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
			const route = claimRoute(claim);
			goto(route.url, route.options);
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
			// Приглашение уже прочитано — возвращаться на него незачем, а его
			// ссылка после входа не действует (C110): заменяем запись истории.
			goto('/auth/code', { replaceState: true });
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}

	function closeMembers() {
		goto(`/invite/${token}`);
	}

	// «Открыть в приложении» (план 66, функция 4): окно принимает адрес
	// схемы wynd:. Строка — только в браузере Windows: в окне мост есть и
	// человек уже в приложении, на телефоне обработчика схемы нет.
	const showOpenInApp = $derived(
		!desktopBridge() && typeof navigator !== 'undefined' && /Windows/.test(navigator.userAgent)
	);

	function openInApp() {
		const url = $page.url.origin + $page.url.pathname + $page.url.search;
		location.href = `wynd:open?url=${encodeURIComponent(url)}`;
	}
</script>

{#if dead}
	<PlainLayout shell app>
		<EmptyState title="Приглашение не действует">
			Ссылку отозвали, она истекла, по ней уже вошли или круг удалён. Попросите новую у того, кто
			вас звал.
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
		<PeekMemberList members={peek.members} />
	</FormLayout>
{:else}
	<PlainLayout app>
		<ScreenTitle class="mt-16">Вас пригласили</ScreenTitle>
		{#if peek && isCircleInvitePeek(peek)}
			<InviteCard
				initial={circleInitial(peek.circle_name)}
				name={peek.circle_name}
				preview={inviteCardPreview(peek)}
			/>
			<Label>Сервер</Label>
			<ServerRow
				name={peek.server_name}
				subtitle={peek.host || displayHost('')}
				card
				gdpr={peek.allow_name_erasure}
				top={0}
			/>
		{:else}
			<InviteCard initial="…" name="…" preview="Загрузка…" />
		{/if}

		<Label>Почта</Label>
		<Input active type="email" autocomplete="email" bind:value={email} />
		<Hint>Пришлём код для входа. Пароля нет.</Hint>
		<Button {loading} disabled={!peek} onclick={onSendCode}>Получить код</Button>

		{#if showOpenInApp}
			<Button variant="ghost" onclick={openInApp}>Открыть в приложении</Button>
		{/if}

		{#if error}
			<Hint class="mt-8">{error}</Hint>
		{/if}
	</PlainLayout>
{/if}

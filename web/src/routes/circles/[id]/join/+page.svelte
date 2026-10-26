<script lang="ts">
	import PeekMemberList from '$ui/data/PeekMemberList.svelte';
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';
	import { getContext, onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { goUp } from '$lib/navigation/up';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import PeopleStrip from '$ui/forms/PeopleStrip.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		clearInviteJoinToken,
		fetchInvitePeek,
		joinViaInvite,
		loadInviteJoinToken,
		memberAvatarColor,
		type InvitePeek
	} from '$lib/auth/invites';
	import {
		declinePendingCircle,
		fetchJoinPreview,
		joinPendingCircle,
		type SplitJoin
	} from '$lib/circles/settings';
	import ConfirmDialog from '$ui/overlays/ConfirmDialog.svelte';
	import Row from '$ui/data/Row.svelte';
	import { formatCutoffDate } from '$lib/format/time';
	import { localDayOf } from '$lib/journal/present';
	import { circleInitial, setCircleIdentity } from '$lib/circles/meta';
	import { rememberCircleOrigin } from '$lib/circles/origin';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { fetchMembers, setIdentityAvatar } from '$lib/circles/settings';
	import IdentityForm from '$ui/forms/IdentityForm.svelte';
	import type { CroppedImage } from '$lib/media/crop';
	import type { MemberInfo } from '$lib/circles/settings';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	const showMembers = $derived($page.url.searchParams.get('members') === '1');

	let peek = $state<InvitePeek | undefined>();
	let members = $state<MemberInfo[]>([]);
	let name = $state('');
	let firstPost = $state('');
	let loading = $state(false);
	let error = $state('');
	let inviteToken = $state<string | undefined>();
	let pendingJoin = $state(false);
	let pendingAvatar = $state<CroppedImage | undefined>();
	let gender = $state<'' | 'm' | 'f'>('');
	// Позвали в отделённый круг (A9, 6.44): имя и прошлое уже есть.
	let split = $state<SplitJoin | undefined>();
	let askDecline = $state(false);
	let declining = $state(false);

	// Отказаться (владелец, 2026-10-25: «Отказаться войти нельзя»).
	async function decline() {
		if (declining) return;
		declining = true;
		try {
			await declinePendingCircle(circle.origin, circle.circleId);
			goto('/circles');
		} catch (err) {
			error = authErrorHint(err);
			askDecline = false;
		} finally {
			declining = false;
		}
	}

	const displayMembers = $derived.by(() => {
		if (peek?.members) {
			return peek.members.slice(0, 5).map((m, i) => ({
				name: m.name,
				initial: circleInitial(m.name),
				color: memberAvatarColor(i)
			}));
		}
		return members
			.filter((m) => m.status === 'active')
			.slice(0, 5)
			.map((m, i) => ({
				name: m.name,
				initial: circleInitial(m.name),
				color: memberAvatarColor(i)
			}));
	});

	const memberTotal = $derived(
		peek?.member_count ?? members.filter((m) => m.status === 'active').length
	);
	const moreCount = $derived(Math.max(0, memberTotal - displayMembers.length));

	onMount(async () => {
		inviteToken = loadInviteJoinToken(circle.circleId);
		try {
			// Одноразовая ссылка к этому экрану уже израсходована (claim при входе),
			// а исчерпанную сервер больше не показывает (SEC-9): тогда круг и тех,
			// кто уже здесь, отдаёт просмотр вступления — без ошибки на экране.
			let peeked = false;
			if (inviteToken) {
				try {
					peek = await fetchInvitePeek(circle.origin, inviteToken);
					peeked = true;
				} catch {
					/* ниже — просмотр вступления */
				}
			}
			if (!peeked) {
				try {
					const preview = await fetchJoinPreview(circle.origin, circle.circleId);
					pendingJoin = true;
					split = preview.split?.you_name ? preview.split : undefined;
					peek = {
						server_name: '',
						host: '',
						circle_name: preview.circle_name,
						color: preview.color,
						member_count: preview.member_count,
						members: preview.members
					};
				} catch {
					members = await fetchMembers(circle.origin, circle.circleId);
				}
			}
		} catch (err) {
			error = authErrorHint(err);
		}
	});

	function openMembers() {
		goto(`/circles/${circle.circleId}/join?members=1`);
	}

	function closeMembers() {
		goto(`/circles/${circle.circleId}/join`);
	}

	async function enterCircle() {
		error = '';
		if (split?.you_name) name = split.you_name;
		const trimmed = name.trim();
		if (!trimmed) {
			error = 'Введите имя';
			return;
		}
		loading = true;
		try {
			if (inviteToken) {
				await joinViaInvite(circle.origin, inviteToken, {
					name: trimmed,
					gender,
					body: firstPost.trim() || undefined
				});
				clearInviteJoinToken(circle.circleId);
			} else if (pendingJoin) {
				await joinPendingCircle(circle.origin, circle.circleId, {
					name: trimmed,
					gender,
					body: firstPost.trim() || undefined
				});
			}
			await setCircleIdentity(circle.origin, circle.circleId, trimmed);
			const avatarOk = pendingAvatar
				? await setIdentityAvatar(circle.origin, circle.circleId, pendingAvatar)
				: true;
			rememberCircleOrigin(circle.circleId, circle.origin);
			circle.identityName = trimmed;
			circle.identityInitial = circleInitial(trimmed);
			await circle.refresh();
			const avatarQuery = !avatarOk ? '?joinAvatar=fail' : '';
			goto(`/circles/${circle.circleId}${avatarQuery}`);
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}

	async function skipToFeed() {
		goto(`/circles/${circle.circleId}`);
	}
</script>

{#if showMembers && peek?.members}
	<FormLayout
		app
		color={circle.color}
		title="Кто уже здесь"
		subtitle={circle.name}
		right={String(peek.member_count)}
		onback={closeMembers}
	>
		<PeekMemberList members={peek.members} />
	</FormLayout>
{:else}
	<!-- Имя можно выбрать и позже: позванный круг стоит в списке кругов строкой
     «Вас позвали — выберите имя», стрелка ведёт туда. -->
	<FormLayout app color={circle.color} circleTitle={circle.name} onback={() => goUp('/circles')}>
		{#if split && pendingJoin}
			<ScreenTitle class="mt-22 lh-125"
				>{split.invite_line || `Круг отделён от круга «${split.split_from}»`}</ScreenTitle
			>
		{/if}
		{#if memberTotal > 0}
			<Label class="mt-16">Кто уже здесь · {memberTotal}</Label>
			<PeopleStrip people={displayMembers} />
			{#if moreCount > 0 && (inviteToken || pendingJoin)}
				<Hint centered class="mt-12">
					<TextButton onclick={openMembers}>ещё {moreCount}</TextButton>
				</Hint>
			{:else if moreCount > 0}
				<Hint centered class="mt-12">ещё {moreCount}</Hint>
			{/if}
		{/if}

		{#if split && pendingJoin}
			{#if split.past_from}
				<Row class="mt-12">
					{#snippet main()}
						<div class="bold">
							Прошлое — с {formatCutoffDate(localDayOf(split?.past_from ?? ''))}
						</div>
						<div class="sub">то, что вы видели в круге «{split?.split_from}»</div>
					{/snippet}
				</Row>
			{/if}
			<Hint>Вы войдёте как {split.you_name}. Круг «{split.split_from}» остаётся как есть.</Hint>
			<Button variant="colored" {loading} onclick={enterCircle}>Войти</Button>
			<Button variant="ghost" class="mt-8" onclick={() => goUp('/circles')}>Не сейчас</Button>
			<Hint centered class="mt-16">
				<TextButton onclick={() => (askDecline = true)}>Отказаться</TextButton>
			</Hint>
		{:else if inviteToken || pendingJoin}
			<IdentityForm
				color={circle.color}
				bind:name
				bind:firstPost
				bind:avatar={pendingAvatar}
				bind:gender
				onerror={(message) => (error = message)}
			/>
			<Button variant="colored" {loading} onclick={enterCircle}>Войти в круг</Button>
		{:else}
			<ScreenTitle class="mt-22 lh-125">
				Добро пожаловать<br />в {circle.name}
			</ScreenTitle>
			<Hint class="mt-12">
				Здесь вас зовут «{circle.identityName}». Первую запись можно сделать в ленте.
			</Hint>
			<Button variant="colored" onclick={skipToFeed}>Войти в круг</Button>
		{/if}

		{#if error}
			<Hint class="mt-12">{error}</Hint>
		{/if}
	</FormLayout>
{/if}

{#if askDecline}
	<ConfirmDialog
		title="Отказаться от «{circle.name}»?"
		confirmLabel="Отказаться"
		onconfirm={() => void decline()}
		oncancel={() => (askDecline = false)}
	>
		<Hint>Круг уйдёт из вашего списка. Позвать снова сможет тот, кто звал.</Hint>
	</ConfirmDialog>
{/if}

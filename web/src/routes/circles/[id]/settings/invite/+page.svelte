<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import NumberField from '$ui/forms/NumberField.svelte';
	import { copyText } from '$lib/clipboard';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import QrCode from '$ui/data/QrCode.svelte';
	import InviteLinkCard from '$ui/data/InviteLinkCard.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		createCircleInvite,
		fetchCircleSettings,
		fetchInviteCandidates,
		patchCircle,
		revokeCircleInvite
	} from '$lib/circles/settings';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import {
		CUSTOM_DAYS_MAX,
		CUSTOM_USES_MAX,
		inviteRequest,
		TTL_PRESETS,
		USES_PRESETS,
		type TtlChoice,
		type UsesChoice
	} from '$lib/circles/invite-options';

	const circle = getContext<CircleContext>(CIRCLE_CTX);
	const fromCreate = $derived($page.url.searchParams.get('from') === 'create');

	function goCircle() {
		goto(`/circles/${circle.circleId}`);
	}

	// После «Создать» круг уже есть: «назад» здесь, «Сначала в круг» ниже и
	// «Готово» на 2.8 ведут в одно место — в круг. Возврат на форму создания
	// или на улочку оставлял гадать, создан ли круг.
	function goBack() {
		if (fromCreate) goCircle();
		else goUp(`/circles/${circle.circleId}/settings`);
	}


	// Опции те же, что у админа на 9.2, без «Без ограничений» и «Без срока»:
	// ссылка участника не шире ссылки админа.
	let kind = $state<'single' | 'multi'>('single');
	let multiUses = $state<UsesChoice>(10);
	let customUses = $state(20);
	let ttl = $state<TtlChoice>(259200);
	let customDays = $state(14);
	let inviteUrl = $state('');
	let error = $state('');
	let loading = $state(false);
	let copied = $state(false);
	let shared = $state(false);
	let currentInviteId = $state('');
	let creating = false;
	let showFromCircles = $state(false);
	let multiInvitesAllowed = $state(false);
	// Кто может менять настройки, видит оба чипа всегда: «Многоразовая»
	// разрешает их в круге, и подпись об этом говорит (2.7, 6.5). Иначе
	// у нового круга выбора не было вовсе — одноразовые стоят с завода.
	let canChangeKinds = $state(false);
	let multiTurnedOn = $state(false);
	const showKindChips = $derived(multiInvitesAllowed || canChangeKinds);

	async function pickKind(next: 'single' | 'multi') {
		if (next === 'multi' && !multiInvitesAllowed) {
			try {
				await patchCircle(circle.origin, circle.circleId, { invite_kind_default: 'multi' });
				multiInvitesAllowed = true;
				multiTurnedOn = true;
			} catch (err) {
				error = authErrorHint(err);
				return;
			}
		}
		kind = next;
		void createLink();
	}

	function pickUses(next: UsesChoice) {
		multiUses = next;
		void createLink();
	}

	function pickTtl(next: TtlChoice) {
		ttl = next;
		void createLink();
	}

	function goFromCircles() {
		const q = fromCreate ? '?from=create' : '';
		goto(`/circles/${circle.circleId}/settings/invite/from${q}`);
	}

	// Ссылка ведёт на сервер круга, а не на хост, с которого открыт клиент:
	// иначе токен круга с сервера B уходил в логи сервера A (аудит 2026-09-22).
	function inviteUrlFor(token: string): string {
		const base = circle.origin || (typeof window !== 'undefined' ? window.location.origin : '');
		return `${base}/invite/${token}`;
	}

	async function createLink() {
		if (creating) return;
		creating = true;
		loading = true;
		error = '';
		copied = false;
		shared = false;
		const previousId = currentInviteId;
		currentInviteId = '';
		try {
			if (previousId) {
				try {
					await revokeCircleInvite(circle.origin, circle.circleId, previousId);
				} catch {
					/* already used or revoked */
				}
			}
			const effectiveKind = multiInvitesAllowed ? kind : 'single';
			const inv = await createCircleInvite(
				circle.origin,
				circle.circleId,
				inviteRequest(effectiveKind === 'single' ? 'single' : multiUses, customUses, ttl, customDays)
			);
			currentInviteId = inv.id;
			inviteUrl = inviteUrlFor(inv.token);
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
			creating = false;
		}
	}

	async function copyLink() {
		if (!inviteUrl) return;
		await copyText(inviteUrl);
		copied = true;
		shared = false;
	}

	async function shareLink() {
		if (!inviteUrl) return;
		if (navigator.share) {
			try {
				await navigator.share({ url: inviteUrl, title: `Приглашение в «${circle.name}»` });
				shared = true;
				copied = false;
			} catch (err) {
				if (err instanceof Error && err.name === 'AbortError') return;
			}
		} else {
			await copyLink();
		}
	}

	onMount(async () => {
		try {
			const settings = await fetchCircleSettings(circle.origin, circle.circleId);
			multiInvitesAllowed = settings.invite_kind_default === 'multi';
			canChangeKinds = settings.is_owner || settings.can_settings;
			const groups = await fetchInviteCandidates(circle.origin, circle.circleId);
			showFromCircles = groups.some((g) => g.members.length > 0);
		} catch {
			/* defaults */
		}
		void createLink();
	});
</script>

<FormLayout
	app
	color={circle.color}
	title="Пригласить в {circle.name}"
	onback={goBack}
>
	{#if showFromCircles}
		<Button class="mt-12" variant="ghost" onclick={goFromCircles}>
			Позвать из других кругов
		</Button>
	{/if}
	<QrCode value={inviteUrl} />
	<Hint class="gutter ctr">Кто ещё не на сервере — код или ссылка</Hint>
	{#if inviteUrl}
		<InviteLinkCard
			url={inviteUrl}
			{shared}
			{copied}
			onshare={() => void shareLink()}
			oncopy={() => void copyLink()}
		/>
	{/if}
	<Label class="mt-22">Ссылка</Label>
	{#if showKindChips}
		<ChipGroup>
			<Chip selected={kind === 'single'} onclick={() => void pickKind('single')}>
				Одноразовая
			</Chip>
			<Chip selected={kind === 'multi'} onclick={() => void pickKind('multi')}>
				Многоразовая
			</Chip>
		</ChipGroup>
	{/if}
	{#if multiInvitesAllowed && kind === 'multi'}
		<ChipGroup style="margin-top:8px">
			{#each USES_PRESETS as n (n)}
				<Chip selected={multiUses === n} onclick={() => pickUses(n)}>{n}</Chip>
			{/each}
			<Chip selected={multiUses === 'custom'} onclick={() => pickUses('custom')}>Своё…</Chip>
		</ChipGroup>
		{#if multiUses === 'custom'}
			<NumberField
				bind:value={customUses}
				min={1}
				max={CUSTOM_USES_MAX}
				onchange={() => void createLink()}
				unit="человек, до {CUSTOM_USES_MAX}"
			/>
		{/if}
	{/if}
	<ChipGroup style="margin-top:8px">
		{#each TTL_PRESETS as opt (opt.sec)}
			<Chip selected={ttl === opt.sec} onclick={() => pickTtl(opt.sec)}>{opt.label}</Chip>
		{/each}
		<Chip selected={ttl === 'custom'} onclick={() => pickTtl('custom')}>Своё…</Chip>
	</ChipGroup>
	{#if ttl === 'custom'}
		<NumberField
			bind:value={customDays}
			min={1}
			max={CUSTOM_DAYS_MAX}
			onchange={() => void createLink()}
			unit="дней, до {CUSTOM_DAYS_MAX}"
		/>
	{/if}
	<!-- Подпись говорит о той ссылке, что на экране: одноразовая — один
	     человек, многоразовая — лимит. Про многоразовые, которых выбрать
	     нельзя, не говорит. -->
	<Hint
		>Ссылка несёт адрес сервера и токен: тому, кого вы зовёте, не придётся ничего вводить.
		{#if multiInvitesAllowed && kind === 'multi'}Многоразовая обязательно имеет лимит — по ней на
			сервер входят новые люди.{:else}По одноразовой войдёт один человек.{/if}{#if multiTurnedOn}{' '}Многоразовые
			ссылки теперь разрешены в круге — выключить можно в настройках.{/if}</Hint
	>
	{#if fromCreate}
		<Button variant="ghost" onclick={goCircle}>Сначала в круг, позову потом</Button>
	{/if}
	{#if error}
		<Hint class="gutter">{error}</Hint>
	{:else if loading}
		<Hint class="gutter">Создание ссылки…</Hint>
	{/if}
</FormLayout>

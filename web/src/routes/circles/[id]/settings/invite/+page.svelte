<script lang="ts">
	import { copyText } from '$lib/clipboard';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import QRCode from 'qrcode';
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import Input from '$ui/forms/Input.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import FieldDisplay from '$ui/forms/FieldDisplay.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Label from '$ui/forms/Label.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		createCircleInvite,
		fetchCircleSettings,
		fetchInviteCandidates,
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
		else goto(`/circles/${circle.circleId}/settings`);
	}


	// Опции те же, что у админа на 9.2, без «Без ограничений» и «Без срока»:
	// ссылка участника не шире ссылки админа.
	let kind = $state<'single' | 'multi'>('single');
	let multiUses = $state<UsesChoice>(10);
	let customUses = $state(20);
	let ttl = $state<TtlChoice>(259200);
	let customDays = $state(14);
	let inviteUrl = $state('');
	let qrSvg = $state('');
	let error = $state('');
	let loading = $state(false);
	let copied = $state(false);
	let shared = $state(false);
	let currentInviteId = $state('');
	let creating = false;
	let showFromCircles = $state(false);
	let multiInvitesAllowed = $state(false);

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

	async function renderQr(url: string) {
		if (!url) {
			qrSvg = '';
			return;
		}
		qrSvg = await QRCode.toString(url, { type: 'svg', margin: 0, width: 142 });
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
			await renderQr(inviteUrl);
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
		<Button variant="ghost" style="margin-top:12px" onclick={goFromCircles}>
			Позвать из других кругов
		</Button>
	{/if}
	{#if qrSvg}
		<div class="qr" style="margin:16px auto 0;width:142px">{@html qrSvg}</div>
	{/if}
	<Hint style="margin:16px;text-align:center">Кто ещё не на сервере — код или ссылка</Hint>
	{#if inviteUrl}
		<FieldDisplay mono value={inviteUrl} style="margin-top:12px;font-size:12.5px;overflow:hidden" />
		<div class="rowin" style="margin-top:12px">
			<Button variant="colored" style="flex:1" onclick={() => void shareLink()}>
				{shared ? 'Отправлено' : 'Поделиться'}
			</Button>
			<Button variant="ghost" style="flex:1;margin:0" onclick={() => void copyLink()}>
				{copied ? 'Скопировано' : 'Скопировать'}
			</Button>
		</div>
	{/if}
	<Label style="margin-top:22px">Ссылка</Label>
	{#if multiInvitesAllowed}
		<ChipGroup>
			<Chip
				selected={kind === 'single'}
				onclick={() => {
					kind = 'single';
					void createLink();
				}}
			>
				Одноразовая
			</Chip>
			<Chip
				selected={kind === 'multi'}
				onclick={() => {
					kind = 'multi';
					void createLink();
				}}
			>
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
			<div class="rowin mt-10">
				<Input
					active
					type="number"
					min="1"
					max={String(CUSTOM_USES_MAX)}
					bind:value={customUses}
					onchange={() => void createLink()}
					class="w72 m-0"
				/>
				<span class="hint m-0">человек, до {CUSTOM_USES_MAX}</span>
			</div>
		{/if}
	{/if}
	<ChipGroup style="margin-top:8px">
		{#each TTL_PRESETS as opt (opt.sec)}
			<Chip selected={ttl === opt.sec} onclick={() => pickTtl(opt.sec)}>{opt.label}</Chip>
		{/each}
		<Chip selected={ttl === 'custom'} onclick={() => pickTtl('custom')}>Своё…</Chip>
	</ChipGroup>
	{#if ttl === 'custom'}
		<div class="rowin mt-10">
			<Input
				active
				type="number"
				min="1"
				max={String(CUSTOM_DAYS_MAX)}
				bind:value={customDays}
				onchange={() => void createLink()}
				class="w72 m-0"
			/>
			<span class="hint m-0">дней, до {CUSTOM_DAYS_MAX}</span>
		</div>
	{/if}
	{#if multiInvitesAllowed}
		<Hint
			>Ссылка несёт адрес сервера и токен: тому, кого вы зовёте, не придётся ничего вводить.
			Многоразовая обязательно имеет лимит — по ней на сервер входят новые люди.</Hint
		>
	{:else}
		<Hint
			>В настройках круга стоят только одноразовые — чипов «Одноразовая / Многоразовая» нет, лимита 5 /
			10 / своё тоже: выбирать не из чего.</Hint
		>
	{/if}
	{#if fromCreate}
		<Button variant="ghost" onclick={goCircle}>Сначала в круг, позову потом</Button>
	{/if}
	{#if error}
		<Hint style="margin:16px">{error}</Hint>
	{:else if loading}
		<Hint style="margin:16px">Создание ссылки…</Hint>
	{/if}
</FormLayout>

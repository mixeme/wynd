<script lang="ts">
	import { copyText } from '$lib/clipboard';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { getContext, onMount } from 'svelte';
	import QRCode from 'qrcode';
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
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

	const circle = getContext<CircleContext>(CIRCLE_CTX);
	const fromCreate = $derived($page.url.searchParams.get('from') === 'create');

	function goCircle() {
		goto(`/circles/${circle.circleId}`);
	}

	function goBack() {
		if (fromCreate) goCircle();
		else goto(`/circles/${circle.circleId}/settings`);
	}

	const TTL_OPTIONS = [
		{ label: '1 час', sec: 3600 },
		{ label: '72 часа', sec: 259200 },
		{ label: 'Неделя', sec: 604800 }
	] as const;

	const MAX_USES_OPTIONS = [5, 10, 25] as const;

	let kind = $state<'single' | 'multi'>('single');
	let maxUses = $state(10);
	let ttlSec = $state(259200);
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
			const inv = await createCircleInvite(circle.origin, circle.circleId, {
				kind: effectiveKind,
				ttl_sec: ttlSec,
				max_uses: effectiveKind === 'single' ? 1 : maxUses
			});
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
			{#each MAX_USES_OPTIONS as uses (uses)}
				<Chip
					selected={maxUses === uses}
					onclick={() => {
						maxUses = uses;
						void createLink();
					}}
				>
					{uses}
				</Chip>
			{/each}
		</ChipGroup>
	{/if}
	<ChipGroup style="margin-top:8px">
		{#each TTL_OPTIONS as opt (opt.sec)}
			<Chip
				selected={ttlSec === opt.sec}
				onclick={() => {
					ttlSec = opt.sec;
					void createLink();
				}}
			>
				{opt.label}
			</Chip>
		{/each}
	</ChipGroup>
	{#if multiInvitesAllowed}
		<Hint
			>Ссылка несёт адрес сервера и токен: тому, кого вы зовёте, не придётся ничего вводить.
			Многоразовая обязательно имеет лимит — по ней на сервер входят новые люди.</Hint
		>
	{:else}
		<Hint
			>В настройках круга стоят только одноразовые — чипов «Одноразовая / Многоразовая» нет, лимита 5 /
			10 / 25 тоже: выбирать не из чего.</Hint
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

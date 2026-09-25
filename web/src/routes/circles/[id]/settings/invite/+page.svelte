<script lang="ts">
	import { goto } from '$app/navigation';
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
	import { createCircleInvite, fetchCircleSettings } from '$lib/circles/settings';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	const TTL_OPTIONS = [
		{ label: '1 час', sec: 3600 },
		{ label: '72 часа', sec: 259200 },
		{ label: 'Неделя', sec: 604800 }
	] as const;

	let kind = $state<'single' | 'multi'>('single');
	let ttlSec = $state(259200);
	let inviteUrl = $state('');
	let qrSvg = $state('');
	let error = $state('');
	let loading = $state(false);

	async function renderQr(url: string) {
		if (!url) {
			qrSvg = '';
			return;
		}
		qrSvg = await QRCode.toString(url, { type: 'svg', margin: 0, width: 142 });
	}

	async function createLink() {
		loading = true;
		error = '';
		try {
			const inv = await createCircleInvite(circle.origin, circle.circleId, {
				kind,
				ttl_sec: ttlSec,
				max_uses: kind === 'single' ? 1 : 10
			});
			const base = typeof window !== 'undefined' ? window.location.origin : '';
			inviteUrl = `${base}/invite/${inv.token}`;
			await renderQr(inviteUrl);
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}

	async function copyLink() {
		if (!inviteUrl) return;
		await navigator.clipboard.writeText(inviteUrl);
	}

	async function shareLink() {
		if (!inviteUrl) return;
		if (navigator.share) {
			await navigator.share({ url: inviteUrl, title: `Приглашение в «${circle.name}»` });
		} else {
			await copyLink();
		}
	}

	onMount(async () => {
		try {
			const settings = await fetchCircleSettings(circle.origin, circle.circleId);
			kind = settings.invite_kind_default ?? 'single';
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
	onback={() => goto(`/circles/${circle.circleId}/settings`)}
>
	{#if qrSvg}
		<div class="qr" style="margin:16px auto 0;width:142px">{@html qrSvg}</div>
	{/if}
	<Hint style="margin:16px;text-align:center">Покажите код или отправьте ссылку</Hint>
	{#if inviteUrl}
		<FieldDisplay mono value={inviteUrl} style="margin-top:12px;font-size:12.5px;overflow:hidden" />
		<div class="rowin" style="margin-top:12px">
			<Button variant="colored" style="flex:1" onclick={() => void shareLink()}>Поделиться</Button>
			<Button variant="ghost" style="flex:1;margin:0" onclick={() => void copyLink()}>
				Скопировать
			</Button>
		</div>
	{/if}
	<Label style="margin-top:22px">Ссылка</Label>
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
	<Hint
		>Ссылка несёт адрес сервера и токен: тому, кого вы зовёте, не придётся ничего вводить. Многоразовая
		обязательно имеет лимит — она создаёт учётки на сервере.</Hint
	>
	{#if error}
		<Hint style="margin:16px">{error}</Hint>
	{:else if loading}
		<Hint style="margin:16px">Создание ссылки…</Hint>
	{/if}
</FormLayout>

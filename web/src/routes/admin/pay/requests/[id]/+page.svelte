<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onMount } from 'svelte';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Button from '$ui/forms/Button.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Icon from '$ui/Icon.svelte';
	import Input from '$ui/forms/Input.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { formatPayDate, formatPayDateTime } from '$lib/pay/pay';
	import {
		approvePayRequest,
		fetchPayRequest,
		rejectPayRequest,
		serverCaption,
		type PayRequest
	} from '$lib/admin/admin';
	import { apiFetch } from '$lib/api/client';
	import { getAdminSession } from '$lib/idb/db';

	const DAY_CHIPS = [
		{ days: 30, label: '30 дней' },
		{ days: 90, label: '3 месяца' },
		{ days: 365, label: 'Год' }
	] as const;

	let request = $state<PayRequest | undefined>();
	let thumb = $state('');
	let days = $state(90);
	let customDays = $state('');
	let custom = $state(false);
	let server = $state('');
	let error = $state('');
	let loading = $state(true);
	let acting = $state(false);

	const id = $derived($page.params.id);

	const selectedDays = $derived.by(() => {
		if (custom) {
			const n = Number(customDays.trim());
			return Number.isInteger(n) && n > 0 ? n : null;
		}
		return days;
	});

	const extendHint = $derived.by(() => {
		if (!request?.subscription_expires_at) return 'Подписка кончилась — считают от сегодня.';
		const end = new Date(request.subscription_expires_at);
		if (Number.isNaN(end.getTime()) || end < new Date()) {
			return 'Подписка кончилась — считают от сегодня.';
		}
		return 'Считают от конца текущей подписки, если она ещё идёт.';
	});

	async function loadThumb(req: PayRequest) {
		if (!req.blob_id || req.blob_deleted) return;
		const session = await getAdminSession();
		if (!session) return;
		const res = await apiFetch(session.origin, `/admin/pay/blob/${req.blob_id}`);
		const blob = await res.blob();
		thumb = URL.createObjectURL(blob);
	}

	async function approve() {
		if (!request || selectedDays == null) {
			error = 'Укажите срок продления';
			return;
		}
		acting = true;
		error = '';
		try {
			await approvePayRequest(request.id, selectedDays);
			goto('/admin/pay/subscription');
		} catch (err) {
			error = authErrorHint(err);
			acting = false;
		}
	}

	async function reject() {
		if (!request) return;
		acting = true;
		error = '';
		try {
			await rejectPayRequest(request.id);
			goto('/admin/pay/subscription');
		} catch (err) {
			error = authErrorHint(err);
			acting = false;
		}
	}

	onMount(() => {
		if (!id) return;
		void (async () => {
			try {
				const [req, caption] = await Promise.all([fetchPayRequest(id), serverCaption()]);
				request = req;
				server = caption;
				await loadThumb(req);
			} catch (err) {
				error = authErrorHint(err);
			} finally {
				loading = false;
			}
		})();
		return () => {
			if (thumb) URL.revokeObjectURL(thumb);
		};
	});
</script>

<AdminWideLayout app active="Оплата" {server}>
	<AdminSection>
		<TextButton
			variant="admin"
			style="font-size:11.5px;color:var(--faint);margin-bottom:8px;display:flex;align-items:center;gap:8px"
			onclick={() => goto('/admin/pay/subscription')}
		>
			<Icon name="back" size="sm" />
			Оплата
		</TextButton>
		{#if loading}
			<Hint>Загрузка…</Hint>
		{:else if error && !request}
			<Hint>{error}</Hint>
		{:else if request}
			<h4 style="margin-bottom:6px">{request.account_email}</h4>
			<div style="font-size:12.5px;color:var(--muted);margin-bottom:18px">
				заявка {formatPayDateTime(request.created_at)}
				{#if request.subscription_expires_at}
					· {new Date(request.subscription_expires_at) < new Date()
						? 'подписка истекла'
						: 'подписка до'}
					{formatPayDate(request.subscription_expires_at)}
				{/if}
			</div>
			<div class="cols">
				<div>
					{#if thumb}
						<img
							src={thumb}
							alt=""
							style="width:100%;border-radius:12px;display:block;aspect-ratio:4/3;object-fit:cover"
						/>
					{/if}
					{#if request.blob_filename}
						<div style="font-size:11.5px;color:var(--faint);margin-top:8px">
							{request.blob_filename}
							{#if request.blob_size_bytes}
								· {Math.round(request.blob_size_bytes / 1024)} КБ
							{/if}
						</div>
					{/if}
				</div>
				<div class="side">
					<SectionLabel style="margin:0 0 8px">Комментарий</SectionLabel>
					<div style="font-size:13.5px;line-height:1.5">
						{request.comment || 'Без комментария'}
					</div>
					<SectionLabel style="margin:22px 0 8px">Продлить на</SectionLabel>
					<ChipGroup style="margin:0">
						{#each DAY_CHIPS as chip (chip.days)}
							<Chip
								selected={!custom && days === chip.days}
								onclick={() => {
									custom = false;
									days = chip.days;
								}}
							>
								{chip.label}
							</Chip>
						{/each}
						<Chip selected={custom} onclick={() => (custom = true)}>Своё…</Chip>
					</ChipGroup>
					{#if custom}
						<Input
							admin
							type="number"
							placeholder="дней"
							style="margin-top:10px;width:120px"
							bind:value={customDays}
						/>
					{/if}
					<div style="font-size:12.5px;color:var(--muted);margin-top:10px;line-height:1.5">
						{extendHint}
					</div>
					<div class="actions">
						<Button disabled={acting} onclick={() => void approve()}>Дать</Button>
						<Button variant="ghost" disabled={acting} onclick={() => void reject()}>Отказать</Button>
					</div>
				</div>
			</div>
			{#if error}
				<Hint style="margin-top:12px">{error}</Hint>
			{/if}
		{/if}
	</AdminSection>
</AdminWideLayout>

<style>
	.cols {
		display: flex;
		gap: 44px;
	}
	.cols > div {
		flex: 1;
	}
	.side {
		display: flex;
		flex-direction: column;
	}
	.actions {
		display: flex;
		gap: 10px;
		margin-top: 18px;
		padding-top: 0;
	}
</style>

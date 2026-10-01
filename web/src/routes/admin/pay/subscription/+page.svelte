<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import DataTable from '$ui/admin/DataTable.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import Icon from '$ui/Icon.svelte';
	import Chip from '$ui/forms/Chip.svelte';
	import ChipGroup from '$ui/forms/ChipGroup.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { formatPayDateTime } from '$lib/pay/pay';
	import {
		fetchPayAccounts,
		fetchPayRequests,
		fetchPaySubscription,
		savePaySubscription,
		serverCaption,
		type PayAccountSummary,
		type PayRequest,
		type PaySubscriptionSettings
	} from '$lib/admin/admin';
	import { subscriptionTableStatus } from '$lib/admin/pay-subscription';
	import { apiFetch } from '$lib/api/client';
	import { getAdminSession } from '$lib/idb/db';

	const REMIND_OPTIONS = [
		{ days: 1, label: 'За сутки' },
		{ days: 3, label: 'За 3 дня' },
		{ days: 7, label: 'За 7 дней' }
	] as const;

	let settings = $state<PaySubscriptionSettings | undefined>();
	let requests = $state<PayRequest[]>([]);
	let accounts = $state<PayAccountSummary[]>([]);
	let thumbs = $state<Record<string, string>>({});
	let server = $state('');
	let error = $state('');
	let loading = $state(true);
	let ready = $state(false);
	let lastSaved = '';

	async function persist() {
		if (!settings) return;
		try {
			await savePaySubscription(settings);
			if (settings.required) {
				requests = await fetchPayRequests();
				accounts = await fetchPayAccounts();
			} else {
				requests = [];
				accounts = [];
			}
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	async function loadThumbs(list: PayRequest[]) {
		const session = await getAdminSession();
		if (!session) return;
		const next: Record<string, string> = {};
		for (const req of list) {
			if (!req.blob_id || req.blob_deleted) continue;
			try {
				const res = await apiFetch(session.origin, `/admin/pay/blob/${req.blob_id}`);
				const blob = await res.blob();
				next[req.id] = URL.createObjectURL(blob);
			} catch {
				/* skip */
			}
		}
		thumbs = next;
	}

	onMount(() => {
		void (async () => {
			try {
				const [sub, caption] = await Promise.all([fetchPaySubscription(), serverCaption()]);
				settings = sub;
				server = caption;
				if (sub.required) {
					const [reqs, accs] = await Promise.all([fetchPayRequests(), fetchPayAccounts()]);
					requests = reqs;
					accounts = accs;
					await loadThumbs(requests);
				}
			} catch (err) {
				error = authErrorHint(err);
			} finally {
				loading = false;
				ready = true;
				lastSaved = JSON.stringify(settings);
			}
		})();
		return () => {
			for (const url of Object.values(thumbs)) URL.revokeObjectURL(url);
		};
	});

	$effect(() => {
		if (!ready || !settings) return;
		const json = JSON.stringify(settings);
		if (json === lastSaved) return;
		lastSaved = json;
		void persist();
	});
</script>

<style>
	.req-row,
	.acc-row {
		cursor: pointer;
	}
</style>

<AdminWideLayout app active="Оплата" {server}>
	<AdminSection>
		<TextButton
			variant="admin"
			class="sz-11 faint mb-8 flex-mid gap-8"
			onclick={() => goto('/admin/pay')}
		>
			<Icon name="back" size="sm" />
			Оплата
		</TextButton>
		<h4 class="mb-8">Подписка</h4>
		{#if loading}
			<Loading compact />
		{:else if error && !settings}
			<Hint>{error}</Hint>
		{:else if settings}
			<div class="flex-top gap-12 mb-18">
				<Switch bind:checked={settings.required} label="Требовать подписку" />
				<div>
					<div class="ttl">Требовать подписку</div>
					<div class="note mt-4 lh-15">
						{#if settings.required}
							Без оплаты круги не открываются.
						{:else}
							Круги открыты. Платить не нужно.
						{/if}
					</div>
				</div>
			</div>
			{#if settings.required}
				<SectionLabel class="mt-0 mx-0 mb-8">Напомнить об истечении</SectionLabel>
				<ChipGroup class="m-0">
					{#each REMIND_OPTIONS as opt (opt.days)}
						<Chip
							selected={settings.remind_days === opt.days}
							onclick={() => {
								settings = { ...settings!, remind_days: opt.days };
							}}
						>
							{opt.label}
						</Chip>
					{/each}
				</ChipGroup>
				<div class="note mt-8 lh-15">
					За столько дней до конца — письмо и строка на улочке. По умолчанию семь.
				</div>
				<SectionLabel class="mt-22 mx-0 mb-8">Заявки</SectionLabel>
				{#if requests.length === 0}
					<Hint>Очередь пуста.</Hint>
				{:else}
					<DataTable>
						<tr>
							<th>Почта</th>
							<th>Когда</th>
							<th>Скрин</th>
							<th>Комментарий</th>
							<th></th>
						</tr>
						{#each requests as req (req.id)}
							<tr
								class="req-row"
								onclick={() => goto(`/admin/pay/requests/${req.id}`)}
							>
								<td class="n">{req.account_email}</td>
								<td class="muted">{formatPayDateTime(req.created_at)}</td>
								<td>
									{#if thumbs[req.id]}
										<img
											src={thumbs[req.id]}
											alt=""
											class="pay-thumb"
										/>
									{/if}
								</td>
								<td class="muted">{req.comment || 'нет'}</td>
								<td class="right">
									<Icon name="chevr" size="sm" />
								</td>
							</tr>
						{/each}
					</DataTable>
				{/if}
				<SectionLabel class="mt-22 mx-0 mb-8">На сервере · {accounts.length}</SectionLabel>
				{#if accounts.length === 0}
					<Hint>Учёток нет.</Hint>
				{:else}
					<DataTable>
						<tr>
							<th>Почта</th>
							<th>Подписка</th>
							<th></th>
						</tr>
						{#each accounts as acc (acc.id)}
							<tr
								class="acc-row"
								onclick={() => goto(`/admin/pay/accounts/${acc.id}`)}
							>
								<td class="n {acc.blocked ? 'faint' : ''}"
									>{acc.email}</td
								>
								<td class="muted">
									{subscriptionTableStatus(acc.subscription_expires_at)}
								</td>
								<td class="right">
									<Icon name="chevr" size="sm" />
								</td>
							</tr>
						{/each}
					</DataTable>
				{/if}
			{:else}
				<div class="note lh-15">
					Напоминания и очереди нет: спрашивать оплату некого. Реквизиты остаются на «Оплате» — ими
					пользуется баннер.
				</div>
			{/if}
			{#if error}
				<Hint class="mt-12">{error}</Hint>
			{/if}
		{/if}
	</AdminSection>
</AdminWideLayout>

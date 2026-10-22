<script lang="ts">
	import SwitchRow from '$ui/admin/SwitchRow.svelte';
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
	import TextButton from '$ui/forms/TextButton.svelte';
	import ConfirmDialog from '$ui/overlays/ConfirmDialog.svelte';
	import PayTermPicker from '$ui/admin/PayTermPicker.svelte';
	import { WORD, plural } from '$lib/format/plural';
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
	import { parseCustomDays, subscriptionTableStatus } from '$lib/admin/pay-subscription';
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
	// Включение подписки (2026-10-21): у кого её нет, круги закрылись бы сразу —
	// перед включением спрашиваем, дать ли всем срок на переход.
	// Срок — те же чипы, что у заявки и учётки (PayTermPicker), плюс «Не давать».
	let askGrant = $state(false);
	let grantDays = $state(90);
	let grantCustom = $state(false);
	let grantCustomDays = $state('');
	let grantUnlimited = $state(false);
	let grantNone = $state(false);
	// Ошибка срока — в самом окне: строка ошибки страницы под затемнением не видна.
	let grantError = $state('');
	let grantChosen = false;

	$effect(() => {
		void [grantDays, grantCustom, grantCustomDays, grantUnlimited, grantNone];
		grantError = '';
	});

	function confirmGrant() {
		if (!settings) return;
		const days = grantNone || grantUnlimited ? 0 : grantCustom ? parseCustomDays(grantCustomDays) : grantDays;
		if (days === null) {
			grantError = 'Укажите число дней';
			return;
		}
		grantChosen = true;
		askGrant = false;
		settings = {
			...settings,
			required: true,
			grant_days: days,
			grant_unlimited: grantUnlimited && !grantNone
		};
	}

	function cancelGrant() {
		askGrant = false;
		if (settings) settings = { ...settings, required: false };
	}

	async function persist() {
		if (!settings) return;
		try {
			await savePaySubscription(settings);
			if (grantChosen) {
				// Включили после вопроса о сроке: срок выдан один раз, счётчик — заново.
				settings = await fetchPaySubscription();
				lastSaved = JSON.stringify(settings);
				grantChosen = false;
			}
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
		const before = lastSaved ? (JSON.parse(lastSaved) as PaySubscriptionSettings) : undefined;
		if (
			settings.required &&
			before &&
			!before.required &&
			!grantChosen &&
			(settings.without_subscription ?? 0) > 0
		) {
			askGrant = true;
			return;
		}
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
			<SwitchRow bind:checked={settings.required} title="Требовать подписку" class="mb-18">
				{#if settings.required}
					Без оплаты круги не открываются.
				{:else}
					Круги открыты. Платить не нужно.
				{/if}
			</SwitchRow>
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
					Придёт письмо и строка в списке кругов.
				</div>
				<SectionLabel class="mt-22 mx-0 mb-8">Заявки</SectionLabel>
				{#if requests.length === 0}
					<Hint>Заявок нет.</Hint>
				{:else}
					<DataTable>
						<tr>
							<th>Почта</th>
							<th>Когда</th>
							<th>Снимок</th>
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
			{/if}
			{#if error}
				<Hint class="mt-12">{error}</Hint>
			{/if}
		{/if}
	</AdminSection>
	{#if askGrant && settings}
		<ConfirmDialog
			title="Включить подписку?"
			confirmLabel="Включить"
			cancelLabel="Не включать"
			onconfirm={confirmGrant}
			oncancel={cancelGrant}
		>
			<Hint class="mx-0">
				Без подписки сейчас {plural(settings.without_subscription ?? 0, WORD.person)}: круги у них
				закроются сразу. Можно дать всем срок на переход.
			</Hint>
			<div class="mt-12">
				<PayTermPicker
					bind:days={grantDays}
					bind:custom={grantCustom}
					bind:customDays={grantCustomDays}
					bind:unlimited={grantUnlimited}
					bind:none={grantNone}
					noneLabel="Не давать"
				/>
			</div>
			{#if grantError}
				<Hint class="mx-0 mt-8">{grantError}</Hint>
			{/if}
		</ConfirmDialog>
	{/if}
</AdminWideLayout>

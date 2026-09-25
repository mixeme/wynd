<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import TextArea from '$ui/forms/TextArea.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { formatPayDate } from '$lib/pay/pay';
	import {
		fetchPayHub,
		savePayRequisites,
		serverCaption,
		type PayHub
	} from '$lib/admin/admin';

	let hub = $state<PayHub | undefined>();
	let requisites = $state('');
	let server = $state('');
	let error = $state('');
	let loading = $state(true);

	function donateSubtitle(data: PayHub): string {
		if (!data.donate.show) return 'выключен';
		const parts = ['баннер в списке кругов'];
		if (data.donate.until) parts.push(`до ${formatPayDate(data.donate.until)}`);
		return parts.join(' · ');
	}

	function subscriptionSubtitle(data: PayHub): string {
		if (!data.subscription.required) return 'выключена';
		const pending = data.subscription.pending_count;
		if (pending > 0) return `требуется · ${pending} заявок ждут`;
		return 'требуется';
	}

	async function persistRequisites() {
		try {
			await savePayRequisites(requisites);
			hub = await fetchPayHub();
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	onMount(async () => {
		try {
			const [data, caption] = await Promise.all([fetchPayHub(), serverCaption()]);
			hub = data;
			requisites = data.requisites;
			server = caption;
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	});
</script>

<AdminWideLayout app active="Оплата" {server}>
	<AdminSection title="Оплата">
		{#if loading}
			<Hint>Загрузка…</Hint>
		{:else if error && !hub}
			<Hint>{error}</Hint>
		{:else if hub}
			<div class="cols">
				<div>
					<SectionLabel style="margin:0 0 8px">Реквизиты</SectionLabel>
					<TextArea
						style="margin:0;height:88px;color:var(--ink)"
						bind:value={requisites}
						onchange={() => void persistRequisites()}
					/>
				</div>
				<div>
					<SectionLabel style="margin:0 0 8px">Как это видит человек</SectionLabel>
					{#if requisites.trim()}
						<div
							style="border:1px solid var(--line);background:var(--card);border-radius:12px;padding:12px 14px"
						>
							<div
								style="font-size:11.5px;letter-spacing:.09em;text-transform:uppercase;font-weight:600;color:var(--muted);margin-bottom:8px"
							>
								Куда платить
							</div>
							<div style="white-space:pre-wrap;font-size:13.5px;line-height:1.5">{requisites}</div>
						</div>
					{:else}
						<Hint>Реквизиты пока пустые — заявку и баннер не показываем.</Hint>
					{/if}
					<div style="font-size:12.5px;color:var(--muted);margin-top:8px;line-height:1.5">
						Когда круги закрыты, когда человек продлевает и когда открывает баннер.
					</div>
				</div>
			</div>
			<SettingsRow
				title="Сбор"
				subtitle={donateSubtitle(hub)}
				onclick={() => goto('/admin/pay/donate')}
				style="margin-top:22px;padding-top:13px;border-top:1px solid var(--line)"
			/>
			<SettingsRow
				title="Подписка"
				subtitle={subscriptionSubtitle(hub)}
				onclick={() => goto('/admin/pay/subscription')}
				style="padding-top:13px;border-top:1px solid var(--line)"
			/>
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
</style>

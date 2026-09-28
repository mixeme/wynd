<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import Label from '$ui/forms/Label.svelte';
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
			<Loading compact />
		{:else if error && !hub}
			<Hint>{error}</Hint>
		{:else if hub}
			<div class="cols">
				<div>
					<SectionLabel class="mt-0 mx-0 mb-8">Реквизиты</SectionLabel>
					<TextArea
						class="m-0 h-88 ink"
						bind:value={requisites}
						onchange={() => void persistRequisites()}
					/>
				</div>
				<div>
					<SectionLabel class="mt-0 mx-0 mb-8">Как это видит человек</SectionLabel>
					{#if requisites.trim()}
						<div class="panel pad-12">
							<Label class="muted mt-0 mx-0 mb-8">Куда платить</Label>
							<div class="pre sz-13 lh-15">{requisites}</div>
						</div>
					{:else}
						<Hint>Реквизиты пока пустые — заявку и баннер не показываем.</Hint>
					{/if}
					<div class="note mt-8 lh-15">
						Когда круги закрыты, когда человек продлевает и когда открывает баннер.
					</div>
				</div>
			</div>
			<SettingsRow
				title="Сбор"
				subtitle={donateSubtitle(hub)}
				onclick={() => goto('/admin/pay/donate')}
				class="mt-22 pt-13 top-line"
			/>
			<SettingsRow
				title="Подписка"
				subtitle={subscriptionSubtitle(hub)}
				onclick={() => goto('/admin/pay/subscription')}
				class="pt-13 top-line"
			/>
			{#if error}
				<Hint class="mt-12">{error}</Hint>
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

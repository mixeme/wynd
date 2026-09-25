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
	import {
		fetchPayAccount,
		fetchPaySubscription,
		grantPayAccount,
		serverCaption,
		type PayAccount
	} from '$lib/admin/admin';
	import { payExtendHint, parseCustomDays, subscriptionAdminSubtitle } from '$lib/admin/pay-subscription';

	const DAY_CHIPS = [
		{ days: 30, label: '30 дней' },
		{ days: 90, label: '3 месяца' },
		{ days: 365, label: 'Год' }
	] as const;

	let account = $state<PayAccount | undefined>();
	let days = $state(90);
	let customDays = $state('');
	let custom = $state(false);
	let unlimited = $state(false);
	let server = $state('');
	let error = $state('');
	let loading = $state(true);
	let acting = $state(false);

	const id = $derived($page.params.id);

	const selectedDays = $derived.by(() => {
		if (unlimited) return null;
		if (custom) {
			return parseCustomDays(customDays);
		}
		return days;
	});

	const extendHint = $derived(payExtendHint(account?.subscription_expires_at, 'grant'));

	async function grant() {
		if (!account) return;
		if (!unlimited && selectedDays == null) {
			error = 'Укажите срок продления';
			return;
		}
		acting = true;
		error = '';
		try {
			if (unlimited) {
				await grantPayAccount(account.id, { unlimited: true });
			} else {
				await grantPayAccount(account.id, { days: selectedDays! });
			}
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
				const sub = await fetchPaySubscription();
				if (!sub.required) {
					await goto('/admin/pay/subscription');
					return;
				}
				const [acc, caption] = await Promise.all([fetchPayAccount(id), serverCaption()]);
				account = acc;
				server = caption;
			} catch (err) {
				error = authErrorHint(err);
			}
			loading = false;
		})();
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
		{:else if error && !account}
			<Hint>{error}</Hint>
		{:else if account}
			<h4 style="margin-bottom:6px">{account.email}</h4>
			<div style="font-size:12.5px;color:var(--muted);margin-bottom:22px">
				{subscriptionAdminSubtitle(account.subscription_expires_at)}
			</div>
			<SectionLabel style="margin:0 0 8px">Продлить на</SectionLabel>
			<ChipGroup style="margin:0">
				{#each DAY_CHIPS as chip (chip.days)}
					<Chip
						selected={!unlimited && !custom && days === chip.days}
						onclick={() => {
							unlimited = false;
							custom = false;
							days = chip.days;
						}}
					>
						{chip.label}
					</Chip>
				{/each}
				<Chip
					selected={!unlimited && custom}
					onclick={() => {
						unlimited = false;
						custom = true;
					}}
				>
					Своё…
				</Chip>
			</ChipGroup>
			<ChipGroup style="margin:8px 0 0">
				<Chip
					selected={unlimited}
					onclick={() => {
						unlimited = true;
						custom = false;
					}}
				>
					Бессрочно
				</Chip>
			</ChipGroup>
			{#if custom && !unlimited}
				<Input
					admin
					type="number"
					placeholder="дней"
					style="margin-top:10px;width:120px"
					bind:value={customDays}
				/>
			{/if}
			<div style="font-size:12.5px;color:var(--muted);margin-top:10px;line-height:1.5;max-width:420px">
				{extendHint}
			</div>
			<div style="margin-top:18px">
				<Button disabled={acting} onclick={() => void grant()}>Дать</Button>
			</div>
			{#if error}
				<Hint style="margin-top:12px">{error}</Hint>
			{/if}
		{/if}
	</AdminSection>
</AdminWideLayout>

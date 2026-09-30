<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import Icon from '$ui/Icon.svelte';
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import RequisitesCard from '$ui/forms/RequisitesCard.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import ShellLayout from '$lib/layouts/ShellLayout.svelte';
	import { displayHost } from '$lib/auth/origin';
	import {
		fetchPayStatus,
		isPayGatewayBlocked,
		isPayGatewayPending,
		PAY_GATEWAY_I_PAID,
		PAY_GATEWAY_LOAD_ERROR,
		PAY_GATEWAY_WAIT,
		payGatewayExpiredHint,
		payGatewayPendingHint,
		payGatewayPendingSubtitle,
		type PayStatus
	} from '$lib/pay/pay';
	import { initSession, loadSessions } from '$lib/session/session.svelte';
	import type { SessionRecord } from '$lib/idb/db';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	let session = $state<SessionRecord | undefined>();
	let status = $state<PayStatus | undefined>();
	let loading = $state(true);
	let gatewayError = $state('');

	const blocked = $derived(isPayGatewayBlocked(status));
	const pending = $derived(isPayGatewayPending(status, blocked));

	onMount(() => {
		void initSession()
			.then(() => loadSessions())
			.then(async (sessions) => {
				if (!sessions.length) {
					goto('/');
					return;
				}
				session = sessions[0];
				try {
					status = await fetchPayStatus(session.origin);
				} catch {
					gatewayError = PAY_GATEWAY_LOAD_ERROR;
				}
			})
			.finally(() => {
				loading = false;
			});
	});
</script>

{#if loading}
	<ShellLayout app>
		<Loading />
	</ShellLayout>
{:else if gatewayError}
	<ShellLayout app>
		<Hint style="margin-top:24px">{gatewayError}</Hint>
	</ShellLayout>
{:else if blocked && status}
	<ShellLayout app>
		<ScreenTitle centered style="margin-top:48px">Доступ закрыт</ScreenTitle>
		{#if pending}
			<Hint centered style="margin:10px 30px 0">{payGatewayPendingHint(status)}</Hint>
			<SectionLabel style="margin-top:22px">Заявка</SectionLabel>
			<SettingsRow
				title="На проверке"
				subtitle={payGatewayPendingSubtitle(status)}
				chevron={false}
				style="padding-top:2px"
			>
				{#snippet control()}
					<Icon name="photo" size="sm" />
				{/snippet}
			</SettingsRow>
			<Button disabled style="margin-top:0" onclick={() => {}}>{PAY_GATEWAY_WAIT}</Button>
		{:else}
			<Hint centered style="margin:10px 30px 0">{payGatewayExpiredHint(status)}</Hint>
			<SectionLabel style="margin-top:22px">Куда платить</SectionLabel>
			<RequisitesCard text={status.requisites} />
			<Button onclick={() => goto('/pay')}>{PAY_GATEWAY_I_PAID}</Button>
		{/if}
		{#if session}
			<Hint centered style="margin-top:22px">
				Вы вошли как {session.email}<br />в «{session.name}» · {displayHost(session.origin)}
			</Hint>
		{/if}
	</ShellLayout>
{:else}
	{@render children()}
{/if}

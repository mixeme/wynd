<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Icon from '$ui/Icon.svelte';
	import ScreenTitle from '$ui/forms/ScreenTitle.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import ShellLayout from '$lib/layouts/ShellLayout.svelte';
	import { displayHost } from '$lib/auth/origin';
	import { fetchPayStatus, formatPayDate, type PayStatus } from '$lib/pay/pay';
	import { initSession, loadSessions } from '$lib/session/session.svelte';
	import type { SessionRecord } from '$lib/idb/db';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	let session = $state<SessionRecord | undefined>();
	let status = $state<PayStatus | undefined>();
	let loading = $state(true);
	let gatewayError = $state('');

	const blocked = $derived(
		status?.required && status.expired && status.has_requisites
	);
	const pending = $derived(blocked && status?.pending);

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
					gatewayError = 'Не удалось проверить доступ. Обновите страницу.';
				}
			})
			.finally(() => {
				loading = false;
			});
	});
</script>

{#if loading}
	<ShellLayout app>
		<Hint style="margin-top:24px">Загрузка…</Hint>
	</ShellLayout>
{:else if gatewayError}
	<ShellLayout app>
		<Hint style="margin-top:24px">{gatewayError}</Hint>
	</ShellLayout>
{:else if blocked}
	<ShellLayout app>
		<ScreenTitle centered style="margin-top:48px">Доступ закрыт</ScreenTitle>
		{#if pending}
			<Hint centered style="margin:10px 30px 0">
				Заявку отправили {status?.pending_at ? formatPayDate(status.pending_at) : ''}. Пока
				администратор не ответит, круги закрыты и новую заявку отправить нельзя.
			</Hint>
			<SectionLabel style="margin-top:22px">Заявка</SectionLabel>
			<SettingsRow
				title="На проверке"
				subtitle="{status?.pending_blob_filename || 'скриншот'}{status?.pending_comment
					? ` · ${status.pending_comment}`
					: ' · без комментария можно было'}"
				chevron={false}
				style="padding-top:2px"
			>
				{#snippet control()}
					<Icon name="photo" size="sm" />
				{/snippet}
			</SettingsRow>
			<Button disabled style="margin-top:0" onclick={() => {}}>Ждём ответа</Button>
		{:else}
			<Hint centered style="margin:10px 30px 0">
				{#if status?.expires_at}
					Подписка на «{status.instance_name || 'сервер'}» закончилась {formatPayDate(
						status.expires_at
					)}. Круги на этом сервере не открываются, пока администратор не подтвердит оплату.
				{:else}
					Подписка на «{status?.instance_name || 'сервер'}» не активна. Круги на этом сервере не
					открываются, пока администратор не подтвердит оплату.
				{/if}
			</Hint>
			<SectionLabel style="margin-top:22px">Куда платить</SectionLabel>
			<div class="req card">{status?.requisites}</div>
			<Button onclick={() => goto('/pay')}>Я оплатил</Button>
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

<style>
	.req {
		margin: 0 16px;
		border: 1px solid var(--line);
		background: var(--card);
		border-radius: 12px;
		padding: 12px 14px;
		white-space: pre-wrap;
		font-size: 13.5px;
		line-height: 1.5;
	}
	.card {
		display: block;
	}
</style>

<script lang="ts">
	// Шлюз оплаты (9.x) вокруг экранов участника: улочка и поиск. Был
	// побайтно скопирован в два +layout (план 47, 2.1) — теперь один.
	// Спрашивает каждый сервер (C67): закрытый закрывает только свои круги
	// (10.16), «Доступ закрыт» одного сервера — по `?closed=` (10.17).
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { goToLogin } from '$lib/navigation/afterLogin';
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
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { displayHost } from '$lib/auth/origin';
	import {
		fetchPayStatus,
		isPayGatewayBlocked,
		isPayGatewayPending,
		PAY_GATEWAY_I_PAID,
		PAY_GATEWAY_WAIT,
		payGatewayExpiredHint,
		payGatewayPendingHint,
		payGatewayPendingSubtitle,
		type PayStatus
	} from '$lib/pay/pay';
	import { serverParam, sessionForParam, setClosedServers } from '$lib/pay/closed.svelte';
	import { initSession, loadSessions } from '$lib/session/session.svelte';
	import type { SessionRecord } from '$lib/idb/db';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	let sessions = $state<SessionRecord[]>([]);
	// Ответ каждого сервера; не ответил — undefined, его круги не закрываем.
	let statuses = $state<Array<PayStatus | undefined>>([]);
	let loading = $state(true);

	const closedIdx = $derived(
		sessions.map((_, i) => i).filter((i) => isPayGatewayBlocked(statuses[i]))
	);
	// Сервер один — шлюз вместо улочки (10.1). Несколько — шлюз сервера из
	// `?closed=`, если он закрыт (10.17); иначе улочка с приглушёнными кругами.
	const single = $derived(sessions.length === 1);
	const gateIdx = $derived.by(() => {
		if (single) return closedIdx.includes(0) ? 0 : -1;
		const wanted = page.url.searchParams.get('closed');
		if (wanted === null) return -1;
		const s = sessionForParam(sessions, wanted);
		const i = s ? sessions.indexOf(s) : -1;
		return closedIdx.includes(i) ? i : -1;
	});
	const session = $derived(gateIdx >= 0 ? sessions[gateIdx] : undefined);
	const status = $derived(gateIdx >= 0 ? statuses[gateIdx] : undefined);
	const pending = $derived(isPayGatewayPending(status, gateIdx >= 0));
	const gateTitle = $derived(page.state.closedTitle ?? session?.name ?? 'Доступ закрыт');

	onMount(() => {
		void initSession()
			.then(() => loadSessions())
			.then(async (loaded) => {
				if (!loaded.length) {
					goToLogin();
					return;
				}
				// Сервер не ответил — не тупик (C63): его круги на улочке скажут,
				// что он не отвечает (7.38, 7.40). Закрытый подпиской круг сервер
				// всё равно не отдаст, а кончившийся вход уже убрал initSession.
				statuses = await Promise.all(
					loaded.map((s) => fetchPayStatus(s.origin).catch(() => undefined))
				);
				sessions = loaded;
				setClosedServers(closedIdx.map((i) => loaded[i].origin));
			})
			.finally(() => {
				loading = false;
			});
	});
</script>

{#snippet gate(s: SessionRecord, st: PayStatus, centered: boolean)}
	<ScreenTitle class={centered ? 'mt-48' : 'mt-28'} {centered}>Доступ закрыт</ScreenTitle>
	{#if pending}
		<Hint class={centered ? 'hint-inset' : undefined} {centered}>{payGatewayPendingHint(st)}</Hint>
		<SectionLabel style="margin-top:22px">Заявка</SectionLabel>
		<SettingsRow
			class="pt-2"
			title="На проверке"
			subtitle={payGatewayPendingSubtitle(st)}
			chevron={false}
		>
			{#snippet control()}
				<Icon name="photo" size="sm" />
			{/snippet}
		</SettingsRow>
		<Button class="mt-0" disabled onclick={() => {}}>{PAY_GATEWAY_WAIT}</Button>
	{:else}
		<Hint class={centered ? 'hint-inset' : undefined} {centered}>{payGatewayExpiredHint(st)}</Hint>
		<SectionLabel style="margin-top:22px">Куда платить</SectionLabel>
		<RequisitesCard text={st.requisites} />
		<Button onclick={() => goto(`/pay?server=${serverParam(s.origin)}`)}
			>{PAY_GATEWAY_I_PAID}</Button
		>
	{/if}
	<Hint class="mt-22" centered>
		Вы вошли как {s.email}<br />в «{s.name}» · {displayHost(s.origin)}
	</Hint>
{/snippet}

{#if loading}
	<ShellLayout app>
		<Loading />
	</ShellLayout>
{:else if session && status && single}
	<ShellLayout app>
		{@render gate(session, status, true)}
	</ShellLayout>
{:else if session && status}
	<FormLayout app title={gateTitle} onback={() => goto('/circles')}>
		{@render gate(session, status, false)}
	</FormLayout>
{:else}
	{@render children()}
{/if}

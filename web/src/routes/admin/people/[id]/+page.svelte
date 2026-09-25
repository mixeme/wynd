<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { formatAdminDay, formatAdminDayYear } from '$lib/format/time';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { CIRCLE_COLORS, type CircleColor } from '$lib/theme/colors';
	import {
		blockAccount,
		deleteAccount,
		fetchAccount,
		serverCaption,
		unblockAccount,
		type AdminAccountDetail
	} from '$lib/admin/admin';
	import { subscriptionPeopleLine } from '$lib/admin/pay-subscription';

	const accountId = $derived($page.params.id ?? '');

	let acc = $state<AdminAccountDetail | undefined>();
	let loginOpen = $state(true);
	let knownOpen = $state(true);
	let server = $state('');
	let error = $state('');
	let loading = $state(true);
	let ready = $state(false);
	let deleting = $state(false);

	function dotColor(color: string): string {
		if (color in CIRCLE_COLORS) return CIRCLE_COLORS[color as CircleColor].cssVar;
		return 'var(--slate)';
	}

	function roleLabel(role: 'owner' | 'member'): string {
		return role === 'owner' ? 'владелец' : 'участник';
	}

	function circleDetail(circle: AdminAccountDetail['circles'][number]): string {
		const role = roleLabel(circle.role);
		if (circle.joined_at) {
			return `${role} · с ${formatAdminDayYear(circle.joined_at)}`;
		}
		return role;
	}

	$effect(() => {
		void loginOpen;
		if (!ready || !acc || loginOpen === knownOpen) return;
		void persistOpen(loginOpen);
	});

	async function persistOpen(open: boolean) {
		if (!acc) return;
		try {
			if (open) await unblockAccount(acc.id);
			else await blockAccount(acc.id);
			knownOpen = open;
			acc.blocked = !open;
		} catch (err) {
			loginOpen = knownOpen;
			error = authErrorHint(err);
		}
	}

	async function removeAccount() {
		if (!acc || acc.owns_circle) return;
		deleting = true;
		error = '';
		try {
			await deleteAccount(acc.id);
			await goto('/admin/people');
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			deleting = false;
		}
	}

	onMount(async () => {
		try {
			const [detail, caption] = await Promise.all([
				fetchAccount(accountId),
				serverCaption()
			]);
			server = caption;
			acc = { ...detail, circles: detail.circles ?? [] };
			loginOpen = !detail.blocked;
			knownOpen = loginOpen;
			ready = true;
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	});
</script>

<AdminWideLayout app active="Люди" {server}>
	<AdminSection>
		<TextButton
			variant="admin"
			class="sz-11 faint mb-8"
			onclick={() => goto('/admin/people')}
		>
			Люди
		</TextButton>
		{#if loading}
			<Hint>Загрузка…</Hint>
		{:else if !acc}
			<Hint>{error || 'Человек не найден'}</Hint>
		{:else}
			<h4 style="margin-bottom:6px">{acc.email}</h4>
			<div class="note mb-20 lh-15">
				на сервере с {formatAdminDayYear(acc.created_at)}
				{#if acc.last_login_at}
					· последний код {formatAdminDay(acc.last_login_at)}
				{/if}
				{#if acc.subscription_required}
					<br />
					{@const payAccountId = acc.id}
					<TextButton
						variant="admin"
						class="note mt-2 p-0 left"
						onclick={() => goto(`/admin/pay/accounts/${payAccountId}`)}
					>
						{subscriptionPeopleLine(acc.subscription_expires_at)}
					</TextButton>
				{/if}
			</div>
			<div class="cols">
				<div>
					<SectionLabel class="mt-0 mx-0 mb-8">Круги · {acc.circles.length}</SectionLabel>
					{#if acc.circles.length === 0}
						<Hint>без кругов</Hint>
					{:else}
						{#each acc.circles as circle (circle.id)}
							<div class="chk">
								<span
									class="dot"
									style="background:{dotColor(circle.color)};margin-top:3px"
								></span>
								<div class="g">
									<div class="n">{circle.name}</div>
									<div class="d">{circleDetail(circle)}</div>
								</div>
							</div>
						{/each}
					{/if}
					<div class="fine mt-10">
						Имён в кругах нет: это лица, не вход на сервер.
						{#if acc.owns_circle}
							Владельца с этой страницы не удалить.
						{:else}
							Владения нет — удалить можно.
						{/if}
					</div>
				</div>
				<div>
					<SectionLabel class="mt-0 mx-0 mb-10">Вход</SectionLabel>
					<div class="flex-top gap-12">
						<Switch bind:checked={loginOpen} class="mt-2" label={loginOpen ? 'Вход открыт' : 'Вход закрыт'} />
						<div>
							<div class="ttl">
								{loginOpen ? 'Вход открыт' : 'Вход закрыт'}
							</div>
							<div class="note mt-4 lh-15">
								Закрыть — код перестанет приходить, круги не трогаются. Открыть можно снова.
							</div>
						</div>
					</div>
					{#if !acc.owns_circle}
						<div class="danger adm-del mt-28 mx-0 mb-0">
							<div class="dl">
								Необратимо
							</div>
							<div class="dt">Удалить с сервера</div>
							<div class="dd note lh-15">
								Записи останутся, события входа и ухода останутся. Имя в круге больше не к чему
								привязать — в хронике будет факт без лица. С этого сервера человек уйдёт.
							</div>
							<div class="dd">
								<TextButton variant="admin" onclick={() => void removeAccount()} disabled={deleting}>
									{deleting ? 'Удаляем…' : 'Удалить'}
								</TextButton>
							</div>
						</div>
					{/if}
					<div class="fine mt-14">
						У кого есть круг во владении, этой кнопки нет: сначала передать владение в круге.
						Панель его не передаёт, подвешенных кругов не бывает.
					</div>
				</div>
			</div>
			{#if error}
				<Hint class="mt-12">{error}</Hint>
			{/if}
		{/if}
	</AdminSection>
</AdminWideLayout>

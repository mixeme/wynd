<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
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

	function formatSince(iso: string): string {
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return iso;
		return new Intl.DateTimeFormat('ru-RU', {
			day: 'numeric',
			month: 'long',
			year: 'numeric'
		}).format(d);
	}

	function formatLogin(iso: string): string {
		const d = new Date(iso);
		if (Number.isNaN(d.getTime())) return iso;
		return new Intl.DateTimeFormat('ru-RU', { day: 'numeric', month: 'long' }).format(d);
	}

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
			return `${role} · с ${formatSince(circle.joined_at)}`;
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
			style="font-size:11.5px;color:var(--faint);margin-bottom:8px"
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
			<div style="font-size:12.5px;color:var(--muted);margin-bottom:20px;line-height:1.5">
				на сервере с {formatSince(acc.created_at)}
				{#if acc.last_login_at}
					· последний код {formatLogin(acc.last_login_at)}
				{/if}
				{#if acc.subscription_required}
					<br />
					{@const payAccountId = acc.id}
					<TextButton
						variant="admin"
						style="font-size:12.5px;color:var(--muted);margin-top:2px;padding:0;text-align:left"
						onclick={() => goto(`/admin/pay/accounts/${payAccountId}`)}
					>
						{subscriptionPeopleLine(acc.subscription_expires_at)}
					</TextButton>
				{/if}
			</div>
			<div class="cols">
				<div>
					<SectionLabel style="margin:0 0 8px">Круги · {acc.circles.length}</SectionLabel>
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
					<div style="font-size:11.5px;color:var(--faint);margin-top:10px;line-height:1.5">
						Имён в кругах нет: это лица, не вход на сервер.
						{#if acc.owns_circle}
							Владельца с этой страницы не удалить.
						{:else}
							Владения нет — удалить можно.
						{/if}
					</div>
				</div>
				<div>
					<SectionLabel style="margin:0 0 10px">Вход</SectionLabel>
					<div style="display:flex;align-items:flex-start;gap:12px">
						<Switch bind:checked={loginOpen} style="margin-top:2px" label={loginOpen ? 'Вход открыт' : 'Вход закрыт'} />
						<div>
							<div style="font-size:13.5px;font-weight:600">
								{loginOpen ? 'Вход открыт' : 'Вход закрыт'}
							</div>
							<div style="font-size:12.5px;color:var(--muted);margin-top:4px;line-height:1.5">
								Закрыть — код перестанет приходить, круги не трогаются. Открыть можно снова.
							</div>
						</div>
					</div>
					{#if !acc.owns_circle}
						<div style="border:1.5px solid var(--ink);border-radius:12px;margin-top:28px">
							<div
								style="font-size:11.5px;letter-spacing:.09em;text-transform:uppercase;font-weight:600;padding:12px 16px 2px"
							>
								Необратимо
							</div>
							<div style="padding:11px 16px 4px;font-weight:600">Удалить с сервера</div>
							<div
								style="padding:0 16px 14px;font-size:12.5px;color:var(--muted);line-height:1.5"
							>
								Записи останутся, события входа и ухода останутся. Имя в круге больше не к чему
								привязать — в хронике будет факт без лица. С этого сервера человек уйдёт.
							</div>
							<div style="padding:0 16px 14px">
								<TextButton variant="admin" onclick={() => void removeAccount()} disabled={deleting}>
									{deleting ? 'Удаляем…' : 'Удалить'}
								</TextButton>
							</div>
						</div>
					{/if}
					<div style="font-size:11.5px;color:var(--faint);margin-top:14px;line-height:1.5">
						У кого есть круг во владении, этой кнопки нет: сначала передать владение в круге.
						Панель его не передаёт, подвешенных кругов не бывает.
					</div>
				</div>
			</div>
			{#if error}
				<Hint style="margin-top:12px">{error}</Hint>
			{/if}
		{/if}
	</AdminSection>
</AdminWideLayout>

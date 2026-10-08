<script lang="ts">
	import CheckRow from '$ui/admin/CheckRow.svelte';
	import DangerNote from '$ui/forms/DangerNote.svelte';
	import SwitchRow from '$ui/admin/SwitchRow.svelte';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { formatAdminDay, formatAdminDayYear } from '$lib/format/time';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import Input from '$ui/forms/Input.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { ApiError } from '$lib/api/client';
	import { INVALID_EMAIL_HINT, isValidParticipantEmail } from '$lib/auth/email';
	import { CIRCLE_COLORS, type CircleColor } from '$lib/theme/colors';
	import {
		blockAccount,
		deleteAccount,
		fetchAccount,
		serverCaption,
		setAccountEmail,
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
	let newEmail = $state('');
	let changingEmail = $state(false);
	let emailNote = $state('');

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

	async function changeEmail() {
		if (!acc) return;
		const trimmed = newEmail.trim();
		emailNote = '';
		if (!isValidParticipantEmail(trimmed)) {
			emailNote = INVALID_EMAIL_HINT;
			return;
		}
		if (trimmed.toLowerCase() === acc.email.toLowerCase()) {
			emailNote = 'Это его нынешняя почта';
			return;
		}
		changingEmail = true;
		try {
			acc.email = await setAccountEmail(acc.id, trimmed);
			newEmail = '';
			emailNote = 'Почта сменена';
		} catch (err) {
			emailNote =
				err instanceof ApiError && err.code === 'conflict'
					? 'Эта почта уже есть на этом сервере'
					: authErrorHint(err);
		} finally {
			changingEmail = false;
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
			<Loading compact />
		{:else if !acc}
			<Hint>{error || 'Человек не найден'}</Hint>
		{:else}
			<h4 class="mb-6">{acc.email}</h4>
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
							<CheckRow
								dotColor={dotColor(circle.color)}
								name={circle.name}
								description={circleDetail(circle)}
							/>
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
					<SectionLabel class="mt-0 mx-0 mb-10">Почта</SectionLabel>
					<div class="flex-mid gap-10">
						<Input
							admin
							class="grow"
							type="email"
							placeholder="новая почта"
							bind:value={newEmail}
						/>
						<TextButton
							variant="adminBox"
							class="bold"
							loading={changingEmail}
							onclick={() => void changeEmail()}>Сменить</TextButton
						>
					</div>
					{#if emailNote}
						<div class="note mt-10">{emailNote}</div>
					{/if}
					<div class="fine mt-10">
						Без кода — для того, кто потерял прежний ящик и сам сменить не может. Убедитесь, что
						просит именно этот человек: с новой почтой он получит все его круги.
					</div>
					<div class="fine mt-8">
						На оба адреса уйдёт письмо. Все его устройства попросят войти заново.
					</div>
					<SectionLabel class="mt-26 mx-0 mb-10">Вход</SectionLabel>
					<SwitchRow bind:checked={loginOpen} title={loginOpen ? 'Вход открыт' : 'Вход закрыт'}>
						Закрыть — код перестанет приходить, круги не трогаются. Открыть можно снова.
					</SwitchRow>
					{#if !acc.owns_circle}
						<DangerNote title="Удалить с сервера" class="mt-28 mx-0 mb-0">
							Записи останутся, события входа и ухода останутся. Имя в круге больше не к чему
							привязать — в хронике будет факт без лица. С этого сервера человек уйдёт.
							{#snippet action()}
								<TextButton variant="admin" onclick={() => void removeAccount()} disabled={deleting}>
									{deleting ? 'Удаляем…' : 'Удалить'}
								</TextButton>
							{/snippet}
						</DangerNote>
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

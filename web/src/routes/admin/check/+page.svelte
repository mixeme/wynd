<script lang="ts">
	import { copyText } from '$lib/clipboard';
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import CheckRow from '$ui/admin/CheckRow.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		CHECK_GROUPS,
		checkHeadline,
		checkSubtitle,
		proxyFixId
	} from '$lib/admin/check-ui';
	import {
		downloadVapidPublicKey,
		runChecks,
		sendAdminPushTest,
		serverCaption,
		type CheckResult
	} from '$lib/admin/admin';

	const LEFT_GROUPS = CHECK_GROUPS.slice(0, 3);
	const RIGHT_GROUPS = CHECK_GROUPS.slice(3);

	let checks = $state<CheckResult[]>([]);
	let server = $state('');
	let error = $state('');
	let loading = $state(true);
	let checkedAt = $state<Date | undefined>();
	// Ошибка действия (пуш, ключ) — у строки или над списком: через error она
	// подменяла весь экран проверки одной строкой.
	let actionError = $state('');
	let pushDetail = $state('сигнал без текста журнала; на loopback может не дойти');
	let pushFailed = $state(false);

	function rowStatus(status: CheckResult['status']): 'ok' | 'warn' | 'bad' {
		if (status === 'fail') return 'bad';
		if (status === 'warn') return 'warn';
		return 'ok';
	}

	const headline = $derived(checkHeadline(checks));
	const subtitle = $derived(checkSubtitle(checks));

	function byId(id: string): CheckResult | undefined {
		if (id === 'push_test') {
			return {
				id: 'push_test',
				status: pushFailed ? 'warn' : 'ok',
				title: 'Тестовый пуш',
				detail: pushDetail
			};
		}
		return checks.find((c) => c.id === id);
	}

	async function load() {
		loading = true;
		error = '';
		try {
			const [list, caption] = await Promise.all([runChecks(), serverCaption()]);
			checks = list;
			server = caption;
			checkedAt = new Date();
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	}

	function checkedLabel(): string {
		if (!checkedAt) return '';
		return `проверено ${checkedAt.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' })}`;
	}

	onMount(() => {
		void load();
	});
</script>

<AdminWideLayout app active="Проверка" {server}>
	<AdminSection>
		{#if loading && !checks.length}
			<Hint>Проверяем инстанс…</Hint>
		{:else if error}
			<Hint>{error}</Hint>
		{:else}
			<div class="flex-top gap-20">
				<div class="grow">
					<h4 style="margin-bottom:5px">{headline}</h4>
					<div class="note">{subtitle}</div>
				</div>
				<div class="right">
					<TextButton variant="adminBox" class="bold" onclick={() => void load()}
						>Проверить снова</TextButton
					>
					<div class="sz-11 faint mt-7">{checkedLabel()}</div>
				</div>
			</div>
			{#if actionError}
				<Hint class="mt-12">{actionError}</Hint>
			{/if}
			<div class="cols mt-20">
				<div>
					{#each LEFT_GROUPS as group, gi (group.label)}
						<SectionLabel class="mx-0 mb-6 {gi === 0 ? 'mt-0' : 'mt-16'}">{group.label}</SectionLabel>
						{#each group.ids as id (id)}
							{@const row = byId(id)}
							{#if row}
								<CheckRow
									status={rowStatus(row.status)}
									bad={row.status === 'fail'}
									name={row.title}
									description={row.detail}
								>
									{#snippet actions()}
										{#if proxyFixId(id) && (row.status === 'fail' || row.status === 'warn')}
											<TextButton variant="admin" onclick={() => goto(`/admin/fix?fail=${id}`)}>
												Показать конфиг
											</TextButton>
										{/if}
									{/snippet}
								</CheckRow>
							{/if}
						{/each}
					{/each}
				</div>
				<div>
					{#each RIGHT_GROUPS as group, gi (group.label)}
						<SectionLabel class="mx-0 mb-6 {gi === 0 ? 'mt-0' : 'mt-16'}">{group.label}</SectionLabel>
						{#each group.ids as id (id)}
							{@const row = byId(id)}
							{#if row}
								<CheckRow
									status={rowStatus(row.status)}
									bad={row.status === 'fail'}
									name={row.title}
									description={row.detail}
								>
									{#snippet actions()}
										{#if id === 'backup' && row.status === 'warn'}
											<TextButton
												variant="admin"
												onclick={async () => {
													await copyText('wynd backup <каталог>');
												}}
											>
												Как настроить
											</TextButton>
										{:else if id === 'vapid_keys'}
											<TextButton
												variant="admin"
												onclick={() =>
													void downloadVapidPublicKey().catch((err) => (actionError = authErrorHint(err)))}
											>
												Скачать копию
											</TextButton>
										{:else if id === 'push_test'}
											<TextButton
												variant="admin"
												onclick={() =>
													void sendAdminPushTest()
														.then(() => {
															pushFailed = false;
															pushDetail = 'отправили на этот браузер';
															return load();
														})
														.catch((err) => {
															pushFailed = true;
															pushDetail = authErrorHint(err);
														})}
											>
												Отправить
											</TextButton>
										{/if}
									{/snippet}
								</CheckRow>
							{/if}
						{/each}
					{/each}
				</div>
			</div>
			<div class="fine mt-18 lh-16">
				Снаружи проверяет ваш браузер: панель просит его сходить на публичный адрес и сравнивает
				с тем, что видит сервер. Wynd никуда не звонит — если браузер окажется в одной сети с
				сервером, здесь будет сказано, что проверка вышла изнутри.
			</div>
		{/if}
	</AdminSection>
</AdminWideLayout>

<script lang="ts">
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
	let pushDetail = $state('сигнал без текста журнала; на loopback может не дойти');

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
				status: 'ok',
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
			<div style="display:flex;align-items:flex-start;gap:20px">
				<div style="flex:1">
					<h4 style="margin-bottom:5px">{headline}</h4>
					<div style="font-size:12.5px;color:var(--muted)">{subtitle}</div>
				</div>
				<div style="text-align:right">
					<TextButton variant="adminBox" style="font-weight:600" onclick={() => void load()}
						>Проверить снова</TextButton
					>
					<div style="font-size:11.5px;color:var(--faint);margin-top:7px">{checkedLabel()}</div>
				</div>
			</div>
			<div class="cols" style="margin-top:20px">
				<div>
					{#each LEFT_GROUPS as group, gi (group.label)}
						<SectionLabel style="margin:{gi === 0 ? '0' : '16px'} 0 6px">{group.label}</SectionLabel>
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
						<SectionLabel style="margin:{gi === 0 ? '0' : '16px'} 0 6px">{group.label}</SectionLabel>
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
													await navigator.clipboard.writeText('wynd backup <каталог>');
												}}
											>
												Как настроить
											</TextButton>
										{:else if id === 'vapid_keys'}
											<TextButton
												variant="admin"
												onclick={() =>
													void downloadVapidPublicKey().catch((err) => (error = authErrorHint(err)))}
											>
												Скачать копию
											</TextButton>
										{:else if id === 'push_test'}
											<TextButton
												variant="admin"
												onclick={() =>
													void sendAdminPushTest()
														.then(() => {
															pushDetail = 'отправили только что';
															return load();
														})
														.catch((err) => (error = authErrorHint(err)))}
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
			<div style="font-size:11.5px;color:var(--faint);margin-top:18px;line-height:1.6">
				Снаружи проверяет ваш браузер: панель просит его сходить на публичный адрес и сравнивает
				с тем, что видит сервер. Wynd никуда не звонит — если браузер окажется в одной сети с
				сервером, здесь будет сказано, что проверка вышла изнутри.
			</div>
		{/if}
	</AdminSection>
</AdminWideLayout>

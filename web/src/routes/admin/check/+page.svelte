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
		fetchVapidPublicKey,
		runChecks,
		sendAdminPushTest,
		serverCaption,
		type CheckResult
	} from '$lib/admin/admin';

	const GROUPS: { label: string; ids: string[] }[] = [
		{ label: 'Снаружи', ids: ['https_outside'] },
		{ label: 'Прокси', ids: ['proxy_headers', 'proxy_body_limit', 'proxy_sse'] },
		{ label: 'Почта', ids: ['smtp', 'dkim'] },
		{ label: 'Пуши', ids: ['vapid_keys'] },
		{ label: 'Сервер', ids: ['clocks', 'disk_space', 'daily_routine', 'backup'] }
	];

	let checks = $state<CheckResult[]>([]);
	let server = $state('');
	let error = $state('');
	let loading = $state(true);
	let checkedAt = $state<Date | undefined>();

	function rowStatus(status: CheckResult['status']): 'ok' | 'warn' | 'bad' {
		if (status === 'fail') return 'bad';
		if (status === 'warn') return 'warn';
		return 'ok';
	}

	const failed = $derived(checks.filter((c) => c.status === 'fail' || c.status === 'warn'));
	const headline = $derived(
		failed.length === 0
			? 'Проверки прошли'
			: failed.length === 1
				? 'Одна проверка не прошла'
				: `${failed.length} проверки не прошли`
	);

	function byId(id: string): CheckResult | undefined {
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
					<div style="font-size:12.5px;color:var(--muted)">
						Статус — формой, а не цветом. Снаружи проверяет этот браузер.
					</div>
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
					{#each GROUPS.slice(0, 2) as group (group.label)}
						<SectionLabel style="margin:{group.label === GROUPS[0].label ? '0' : '16px'} 0 6px"
							>{group.label}</SectionLabel
						>
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
										{#if id.startsWith('proxy') && (row.status === 'fail' || row.status === 'warn')}
											<TextButton variant="admin" onclick={() => goto(`/admin/fix?fail=${id}`)}>
												Показать конфиг
											</TextButton>
										{:else if id === 'smtp'}
											<TextButton variant="admin" onclick={() => goto('/admin/smtp')}>
												Настроить
											</TextButton>
										{/if}
									{/snippet}
								</CheckRow>
							{/if}
						{/each}
					{/each}
				</div>
				<div>
					{#each GROUPS.slice(2) as group (group.label)}
						<SectionLabel style="margin:{group.label === 'Почта' ? '0' : '16px'} 0 6px"
							>{group.label}</SectionLabel
						>
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
										{#if id === 'smtp'}
											<TextButton variant="admin" onclick={() => goto('/admin/smtp')}>
												Настроить
											</TextButton>
										{:else if id === 'dkim' && row.status !== 'na'}
											<TextButton variant="admin" onclick={() => goto('/admin/smtp')}>
												Как добавить
											</TextButton>
										{:else if id === 'backup' && row.status === 'warn'}
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
												onclick={async () => {
													const key = await fetchVapidPublicKey();
													await navigator.clipboard.writeText(key);
												}}
											>
												Скопировать ключ
											</TextButton>
										{/if}
									{/snippet}
								</CheckRow>
							{/if}
						{/each}
					{/each}
					<SectionLabel style="margin:16px 0 6px">Тестовый пуш</SectionLabel>
					<CheckRow
						status="ok"
						name="Тестовый пуш"
						description="сигнал без текста журнала; на loopback может не дойти"
					>
						{#snippet actions()}
							<TextButton
								variant="admin"
								onclick={() =>
									void sendAdminPushTest().catch((err) => (error = authErrorHint(err)))}
							>
								Отправить
							</TextButton>
						{/snippet}
					</CheckRow>
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

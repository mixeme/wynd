<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Icon from '$ui/Icon.svelte';
	import Input from '$ui/forms/Input.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import TextArea from '$ui/forms/TextArea.svelte';
	import TextButton from '$ui/forms/TextButton.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { fetchPayDonate, savePayDonate, serverCaption, type PayDonateSettings } from '$lib/admin/admin';

	let settings = $state<PayDonateSettings | undefined>();
	let server = $state('');
	let error = $state('');
	let loading = $state(true);
	let ready = $state(false);
	let lastSaved = '';
	let persistTimer: ReturnType<typeof setTimeout> | undefined;

	async function persist() {
		if (!settings) return;
		try {
			await savePayDonate(settings);
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	onMount(async () => {
		try {
			const [data, caption] = await Promise.all([fetchPayDonate(), serverCaption()]);
			settings = data;
			server = caption;
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
			ready = true;
			lastSaved = JSON.stringify(settings);
		}
	});

	$effect(() => {
		if (!ready || !settings) return;
		const json = JSON.stringify(settings);
		if (json === lastSaved) return;
		clearTimeout(persistTimer);
		persistTimer = setTimeout(() => {
			lastSaved = json;
			void persist();
		}, 500);
		return () => clearTimeout(persistTimer);
	});
</script>

<AdminWideLayout app active="Оплата" {server}>
	<AdminSection>
		<TextButton
			variant="admin"
			style="font-size:11.5px;color:var(--faint);margin-bottom:8px;display:flex;align-items:center;gap:8px"
			onclick={() => goto('/admin/pay')}
		>
			<Icon name="back" size="sm" />
			Оплата
		</TextButton>
		<h4 style="margin-bottom:8px">Сбор</h4>
		{#if loading}
			<Hint>Загрузка…</Hint>
		{:else if error && !settings}
			<Hint>{error}</Hint>
		{:else if settings}
			<div class="cols">
				<div>
					<SectionLabel style="margin:0 0 8px">Баннер в списке кругов</SectionLabel>
					<TextArea style="margin:0;height:88px;color:var(--ink)" bind:value={settings.text} />
					<div style="font-size:12.5px;color:var(--muted);margin-top:8px;line-height:1.5">
						Без сумм и без прогресса. Один баннер.
					</div>
				</div>
				<div>
					<div style="display:flex;align-items:flex-start;gap:12px">
						<Switch bind:checked={settings.show} />
						<div>
							<div style="font-size:13.5px;font-weight:600">Показывать</div>
							<div style="font-size:12.5px;color:var(--muted);margin-top:4px;line-height:1.5">
								Выключить можно раньше срока. Баннер не связан с подпиской.
							</div>
						</div>
					</div>
					<div style="display:flex;align-items:flex-start;gap:12px;margin-top:18px">
						<Switch bind:checked={settings.dismissible} />
						<div>
							<div style="font-size:13.5px;font-weight:600">Можно скрыть</div>
							<div style="font-size:12.5px;color:var(--muted);margin-top:4px;line-height:1.5">
								Скрытие постоянное, пока не выйдет новый баннер.
							</div>
						</div>
					</div>
					<div style="display:flex;align-items:center;gap:10px;margin-top:18px">
						<span style="font-size:12.5px">До</span>
						<Input
							admin
							type="date"
							style="width:180px"
							bind:value={settings.until}
						/>
					</div>
				</div>
			</div>
			{#if error}
				<Hint style="margin-top:12px">{error}</Hint>
			{/if}
		{/if}
	</AdminSection>
</AdminWideLayout>

<style>
	.cols {
		display: flex;
		gap: 44px;
	}
	.cols > div {
		flex: 1;
	}
</style>

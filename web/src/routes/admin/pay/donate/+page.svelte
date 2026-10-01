<script lang="ts">
	import SwitchRow from '$ui/admin/SwitchRow.svelte';
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import Icon from '$ui/Icon.svelte';
	import Input from '$ui/forms/Input.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
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
			class="sz-11 faint mb-8 flex-mid gap-8"
			onclick={() => goto('/admin/pay')}
		>
			<Icon name="back" size="sm" />
			Оплата
		</TextButton>
		<h4 class="mb-8">Сбор</h4>
		{#if loading}
			<Loading compact />
		{:else if error && !settings}
			<Hint>{error}</Hint>
		{:else if settings}
			<div class="cols">
				<div>
					<SectionLabel class="mt-0 mx-0 mb-8">Баннер в списке кругов</SectionLabel>
					<TextArea class="m-0 h-88 ink" bind:value={settings.text} />
					<div class="note mt-8 lh-15">
						Без сумм и без прогресса. Один баннер.
					</div>
				</div>
				<div>
					<SwitchRow bind:checked={settings.show} title="Показывать">
						Выключить можно раньше срока. Баннер не связан с подпиской.
					</SwitchRow>
					<SwitchRow bind:checked={settings.dismissible} title="Можно скрыть" class="mt-18">
						Скрытие постоянное, пока не выйдет новый баннер.
					</SwitchRow>
					<div class="flex-mid gap-10 mt-18">
						<span class="sz-12">До</span>
						<Input
							admin
							type="date"
							class="w180"
							bind:value={settings.until}
						/>
						<!-- Поле даты на телефоне не очищается — срок снимает своя ссылка. -->
						{#if settings.until}
							<TextButton class="sz-12" onclick={() => settings && (settings.until = '')}
								>убрать срок</TextButton
							>
						{:else}
							<span class="note">без срока — пока не выключите</span>
						{/if}
					</div>
				</div>
			</div>
			{#if error}
				<Hint class="mt-12">{error}</Hint>
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

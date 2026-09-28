<script lang="ts">
	import { onMount } from 'svelte';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import Input from '$ui/forms/Input.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import {
		fetchCompression,
		saveCompression,
		serverCaption,
		type CompressionSettings
	} from '$lib/admin/admin';

	let settings = $state<CompressionSettings | undefined>();
	let attachmentMb = $state(100);
	let videoBitrateMbps = $state(6);
	let server = $state('');
	let error = $state('');
	let loading = $state(true);

	function syncAttachmentMb() {
		if (!settings) return;
		attachmentMb = Math.round(settings.attachment_max_bytes / (1024 * 1024));
	}

	function syncVideoBitrateMbps() {
		if (!settings) return;
		videoBitrateMbps = settings.video_bitrate_kbps / 1000;
	}

	async function persistVideoBitrateMbps() {
		if (!settings) return;
		const mbps = Math.max(1, Math.round(videoBitrateMbps));
		videoBitrateMbps = mbps;
		settings.video_bitrate_kbps = mbps * 1000;
		await persist();
	}

	async function persistAttachmentMb() {
		if (!settings) return;
		const mb = Math.max(1, Math.round(attachmentMb));
		attachmentMb = mb;
		settings.attachment_max_bytes = mb * 1024 * 1024;
		await persist();
	}

	async function persist() {
		if (!settings) return;
		try {
			await saveCompression(settings);
		} catch (err) {
			error = authErrorHint(err);
		}
	}

	onMount(async () => {
		try {
			const [cs, caption] = await Promise.all([fetchCompression(), serverCaption()]);
			settings = cs;
			syncAttachmentMb();
			syncVideoBitrateMbps();
			server = caption;
		} catch (err) {
			error = authErrorHint(err);
		} finally {
			loading = false;
		}
	});
</script>

<AdminWideLayout app active="Сжатие" {server}>
	<AdminSection title="Сжатие и вложения">
		{#if loading}
			<Loading compact />
		{:else if error && !settings}
			<Hint>{error}</Hint>
		{:else if settings}
			<div class="flex gap-44">
				<div class="grow">
					<SectionLabel class="mt-0 mx-0 mb-10">Фотографии</SectionLabel>
					<div class="flex-mid gap-10 mb-10">
						<span class="sz-12 w120 nowrap">Длинная сторона</span>
						<Input
							admin
							class="w88"
							type="number"
							bind:value={settings.photo_max_px}
							onchange={() => void persist()}
						/>
						<span class="note">px</span>
					</div>
					<div class="flex-mid gap-10 mb-10">
						<span class="sz-12 w120 nowrap">Формат и качество</span>
						<Input
							admin
							class="w88"
							type="number"
							bind:value={settings.photo_quality}
							onchange={() => void persist()}
						/>
						<span class="note">WebP</span>
					</div>
					<SectionLabel class="mt-22 mx-0 mb-10">Видео</SectionLabel>
					<div class="flex-mid gap-10 mb-10">
						<span class="sz-12 w120 nowrap">Разрешение</span>
						<Input
							admin
							class="w88"
							type="number"
							bind:value={settings.video_max_height}
							onchange={() => void persist()}
						/>
						<span class="note">p</span>
					</div>
					<div class="flex-mid gap-10">
						<span class="sz-12 w120 nowrap">Битрейт</span>
						<Input
							admin
							class="w88"
							type="number"
							bind:value={videoBitrateMbps}
							onchange={() => void persistVideoBitrateMbps()}
						/>
						<span class="note">Мбит/с</span>
					</div>
					<SectionLabel class="mt-22 mx-0 mb-10">Файлы</SectionLabel>
					<div class="flex-mid gap-10">
						<span class="sz-12 w120 nowrap">Потолок размера</span>
						<Input
							admin
							class="w88"
							type="number"
							bind:value={attachmentMb}
							onchange={() => void persistAttachmentMb()}
						/>
						<span class="note">МБ</span>
					</div>
				</div>
				<div class="grow">
					<div class="panel sz-13" style="padding:16px 18px">
						Wynd хранит то, что сказано, а не вашу медиатеку. Оригиналы остаются на телефоне.
					</div>
					<div class="note mt-16 lh-16">
						Дефолты щедрые, а не экономные: сжатие необратимо, оригинал сюда не приезжает. Если
						через год качества окажется мало, переделать будет нечего — экономия должна быть
						осознанным выбором.
					</div>
					<div class="note mt-14 lh-16">
						Изменение настроек не трогает загруженное. В круге будут соседствовать записи разного
						качества, и это нормально.
					</div>
					<div class="flex-mid gap-12 mt-20">
						<Switch checked={true} disabled label="Отдавать вложения только как загрузку" />
						<span class="sz-12"
							>Отдавать вложения только как загрузку<br /><span class="faint"
								>Content-Disposition: attachment · выключать нельзя</span
							></span
						>
					</div>
				</div>
			</div>
			{#if error}
				<Hint class="mt-16">{error}</Hint>
			{/if}
		{/if}
	</AdminSection>
</AdminWideLayout>

<script lang="ts">
	import { onMount } from 'svelte';
	import AdminSection from '$ui/admin/AdminSection.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Input from '$ui/forms/Input.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import Switch from '$ui/forms/Switch.svelte';
	import AdminWideLayout from '$lib/layouts/AdminWideLayout.svelte';
	import { authErrorHint } from '$lib/auth/auth';
	import { formatBytes } from '$lib/format/bytes';
	import {
		fetchCompression,
		saveCompression,
		serverCaption,
		type CompressionSettings
	} from '$lib/admin/admin';

	let settings = $state<CompressionSettings | undefined>();
	let server = $state('');
	let error = $state('');
	let loading = $state(true);

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
			<Hint>Загрузка…</Hint>
		{:else if error && !settings}
			<Hint>{error}</Hint>
		{:else if settings}
			<div style="display:flex;gap:44px">
				<div style="flex:1">
					<SectionLabel style="margin:0 0 10px">Фотографии</SectionLabel>
					<div style="display:flex;align-items:center;gap:10px;margin-bottom:10px">
						<span style="font-size:12.5px;width:110px">Длинная сторона</span>
						<Input
							admin
							style="width:88px"
							type="number"
							bind:value={settings.photo_max_px}
							onchange={() => void persist()}
						/>
						<span style="font-size:12.5px;color:var(--muted)">px</span>
					</div>
					<div style="display:flex;align-items:center;gap:10px;margin-bottom:10px">
						<span style="font-size:12.5px;width:110px">Качество JPEG</span>
						<Input
							admin
							style="width:88px"
							type="number"
							bind:value={settings.photo_quality}
							onchange={() => void persist()}
						/>
					</div>
					<SectionLabel style="margin:22px 0 10px">Видео</SectionLabel>
					<div style="display:flex;align-items:center;gap:10px;margin-bottom:10px">
						<span style="font-size:12.5px;width:110px">Разрешение</span>
						<Input
							admin
							style="width:88px"
							type="number"
							bind:value={settings.video_max_height}
							onchange={() => void persist()}
						/>
						<span style="font-size:12.5px;color:var(--muted)">px</span>
					</div>
					<div style="display:flex;align-items:center;gap:10px">
						<span style="font-size:12.5px;width:110px">Битрейт</span>
						<Input
							admin
							style="width:88px"
							type="number"
							bind:value={settings.video_bitrate_kbps}
							onchange={() => void persist()}
						/>
						<span style="font-size:12.5px;color:var(--muted)">кбит/с</span>
					</div>
					<SectionLabel style="margin:22px 0 10px">Файлы</SectionLabel>
					<div style="display:flex;align-items:center;gap:10px">
						<span style="font-size:12.5px;width:110px">Потолок размера</span>
						<Input
							admin
							style="width:88px"
							value={formatBytes(settings.attachment_max_bytes)}
							readonly
						/>
					</div>
				</div>
				<div style="flex:1">
					<div
						style="border:1px solid var(--line);background:var(--card);border-radius:12px;padding:16px 18px;font-size:13.5px"
					>
						Wynd хранит то, что сказано, а не вашу медиатеку. Оригиналы остаются на телефоне.
					</div>
					<div style="font-size:12.5px;color:var(--muted);margin-top:16px;line-height:1.6">
						Дефолты щедрые, а не экономные: сжатие необратимо, оригинал сюда не приезжает. Если
						через год качества окажется мало, переделать будет нечего — экономия должна быть
						осознанным выбором.
					</div>
					<div style="font-size:12.5px;color:var(--muted);margin-top:14px;line-height:1.6">
						Изменение настроек не трогает загруженное. В круге будут соседствовать записи разного
						качества, и это нормально.
					</div>
					<div style="display:flex;align-items:center;gap:12px;margin-top:20px">
						<Switch checked={true} disabled />
						<span style="font-size:12.5px"
							>Отдавать вложения только как загрузку<br /><span style="color:var(--faint)"
								>Content-Disposition: attachment · выключать нельзя</span
							></span
						>
					</div>
				</div>
			</div>
			{#if error}
				<Hint style="margin-top:16px">{error}</Hint>
			{/if}
		{/if}
	</AdminSection>
</AdminWideLayout>

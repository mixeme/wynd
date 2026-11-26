<script lang="ts">
	import FormLayout from '$lib/layouts/FormLayout.svelte';
	import { navigateBack } from '$lib/navigation/up';
	import Button from '$ui/forms/Button.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import SectionLabel from '$ui/data/SectionLabel.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import { ANDROID_RELEASES_PAGE, fetchLatestAndroidAppRelease } from '$lib/android/appRelease';

	// Установка приложения Wynd для Android (C139). Баннер на улочке ведёт
	// сюда: что даёт приложение, шаги установки и одна кнопка.
	// Адрес файла берём в момент нажатия — выпуск всегда самый новый;
	// не вышло — ведём на страницу выпусков, там файл виден человеку.

	let resolving = $state(false);

	async function download() {
		if (resolving) return;
		resolving = true;
		const release = await fetchLatestAndroidAppRelease();
		resolving = false;
		location.href = release?.apkUrl ?? ANDROID_RELEASES_PAGE;
	}
</script>

<FormLayout shell app title="Приложение для Android" onback={navigateBack}>
	<Hint
		>Записи уходят, даже если окно закрыто. Уведомления приходят, когда приложение не открыто.</Hint
	>
	<SectionLabel>Что даёт</SectionLabel>
	<SettingsRow
		icon="bell"
		title="Уведомления"
		subtitle="О новых записях и откликах"
		chevron={false}
		divided="both"
	/>
	<SettingsRow
		icon="send"
		title="Записи в фоне"
		subtitle="Окно можно закрыть"
		chevron={false}
		divided="bottom"
	/>
	<SettingsRow
		icon="video"
		title="Видео сжимается на телефоне"
		subtitle="Экран держать открытым не нужно"
		chevron={false}
		divided="bottom"
	/>
	<SectionLabel>Установка</SectionLabel>
	<Hint>Скачайте файл и откройте его. Android спросит разрешение на установку — разрешите.</Hint>
	<Button loading={resolving} loadingLabel="Ищем файл…" onclick={() => void download()}>
		Скачать приложение
	</Button>
	<Hint>Потом войдите: адрес своего сервера, почта и код из письма.</Hint>
</FormLayout>

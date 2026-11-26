<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { navigateBack } from '$lib/navigation/up';
	import { detailEntryOptions } from '$lib/navigation/detailEntry';
	import SettingsRow from '$ui/data/SettingsRow.svelte';
	import FormLayout from '$lib/layouts/FormLayout.svelte';

	// План 76: на широком окне настройки круга — левая колонка; справа
	// открывается выбранный раздел. В меню только пункты, ведущие на экран.
	// Значения (ссылки, имя в круге, участники, архив) остаются в разделах.
	const id = $derived(page.params.id ?? '');
	const path = $derived(page.url.pathname.replace(/\/+$/, '') || '/');
	const root = $derived(`/circles/${id}/settings`);
	const hasDetail = $derived(path !== root);

	function under(base: string): boolean {
		return path === base || path.startsWith(`${base}/`);
	}

	function open(target: string) {
		if (path === target) return;
		goto(target, detailEntryOptions(hasDetail));
	}

	function goBack() {
		// Левая колонка — выход из настроек в сам круг; верхняя стрелка
		// раздела справа остаётся шагом на уровень выше.
		if (id) void goto(`/circles/${id}`, { replaceState: true });
		else void navigateBack();
	}
</script>

<FormLayout shell app title="Настройки круга" onback={goBack}>
	<SettingsRow
		class="mt-8"
		title="Настройки круга"
		selected={path === root}
		onclick={() => open(root)}
	/>
	<SettingsRow
		title="Пригласить"
		selected={under(`${root}/invite`)}
		onclick={() => open(`${root}/invite`)}
	/>
	<SettingsRow
		title="Живые ссылки"
		selected={under(`${root}/invites`)}
		onclick={() => open(`${root}/invites`)}
	/>
	<SettingsRow
		title="Кто вы в этом круге"
		selected={under(`${root}/identity`)}
		onclick={() => open(`${root}/identity`)}
	/>
	<SettingsRow
		title="Уведомления"
		selected={under(`${root}/notify`)}
		onclick={() => open(`${root}/notify`)}
	/>
	<SettingsRow
		title="Место и архив"
		selected={under(`/circles/${id}/quota`)}
		onclick={() => open(`/circles/${id}/quota`)}
	/>
	<SettingsRow
		title="Свой архив"
		selected={path === `/circles/${id}/archive`}
		onclick={() => open(`/circles/${id}/archive`)}
	/>
	<SettingsRow
		title="Отделиться"
		selected={path === `/circles/${id}/split`}
		onclick={() => open(`/circles/${id}/split`)}
	/>
	<SettingsRow
		title="Участники"
		selected={under(`${root}/members`)}
		onclick={() => open(`${root}/members`)}
	/>
	<SettingsRow
		title="Перенести на другой сервер"
		selected={path === `/circles/${id}/move`}
		onclick={() => open(`/circles/${id}/move`)}
	/>
</FormLayout>

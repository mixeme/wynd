<script lang="ts">
	import Hint from '$ui/forms/Hint.svelte';
	import { appVersion } from '$lib/appinfo';
	import { sourceUrl } from '$lib/instance/source.svelte';

	// Подвал «Wynd x.y · AGPL-3.0 · исходный код · лицензии» (план 47, 2.21):
	// низ настроек приложения и панели администратора. Адрес исходников
	// экран загружает сам (loadSourceUrl) — подвал его только показывает.
	let {
		wrap = false,
		class: className = ''
	}: {
		/** Ссылки второй строкой (узкий экран телефона). */
		wrap?: boolean;
		class?: string;
	} = $props();
</script>

<Hint centered class={className}>
	Wynd {appVersion} · AGPL-3.0{#if wrap}<br />{:else}{' · '}{/if}
	<a class="under" href={sourceUrl()}>исходный код</a> ·
	<a class="under" href="/THIRD_PARTY_LICENSES.txt">лицензии компонентов</a>
</Hint>

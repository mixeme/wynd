<script lang="ts">
	import type { Snippet } from 'svelte';

	/**
	 * Общая основа строк `row2`: ведущий элемент, текст `.g`, хвост.
	 * С `onclick` строка — `<button type="button">`, без него — `<div>`.
	 * Раньше каждая строка писала обе ветки целиком, и разметка жила в
	 * двух копиях (план 42, UI-5). Экраны зовут не её, а именные строки
	 * (`SettingsRow`, `ServerRow`, `MemberRow`, `SearchResultRow`) — их
	 * имена и параметры повторяют словарь кадров screens.html.
	 */
	let {
		onclick,
		link = false,
		opacity,
		class: className = '',
		style = '',
		leading,
		main,
		trailing
	}: {
		onclick?: () => void;
		/** `.g.link` — строка-ссылка («Выйти», «Все участники»). */
		link?: boolean;
		opacity?: number;
		class?: string;
		style?: string;
		leading?: Snippet;
		main: Snippet;
		trailing?: Snippet;
	} = $props();
</script>

{#snippet inner()}
	{@render leading?.()}
	<div class="g" class:link>{@render main()}</div>
	{@render trailing?.()}
{/snippet}

<!-- Внешний тег — две короткие ветки, а не <svelte:element>: у того Svelte
     требует явную роль при обработчике, и кнопка получила бы лишний role. -->
{#if onclick}
	<button type="button" class="row2 {className}" style:opacity {style} {onclick}>
		{@render inner()}
	</button>
{:else}
	<div class="row2 {className}" style:opacity {style}>
		{@render inner()}
	</div>
{/if}

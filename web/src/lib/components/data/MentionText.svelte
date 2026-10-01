<script lang="ts">
	import { splitMentionBody } from '$lib/journal/mentions';

	// Текст записи или комментария с упоминаниями (план 47, 2.7): @имя —
	// цветом круга, без ссылки (у участника нет страницы). Переносы и пустые
	// строки — как их написали: pre-wrap. Разметка в одну строку нарочно —
	// с pre-wrap любой перенос шаблона стал бы видимой пустой строкой.
	let { body, class: className = '' }: { body: string; class?: string } = $props();
</script>

<span class="mention-text {className}"
	>{#each splitMentionBody(body) as part, i (i)}{#if part.kind === 'mention'}<span class="men"
				>{part.value}</span
			>{:else}{part.value}{/if}{/each}</span
>

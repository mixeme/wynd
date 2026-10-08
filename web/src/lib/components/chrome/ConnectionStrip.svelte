<script lang="ts">
	import Icon from '$ui/Icon.svelte';
	import { plural, WORD } from '$lib/format/plural';
	import type { ConnectionNotice } from '$lib/session/connection.svelte';

	// Полоса связи под шапкой (7.6–7.8). На связи её нет вовсе (7.5): знак
	// появляется, когда что-то не так, и уходит сам. Цвета бумаги, без
	// красного — подвал или метро, а не авария.
	let { notice }: { notice: ConnectionNotice | undefined } = $props();

	function sendingText(posts: number, comments: number): string {
		const parts = [];
		if (posts) parts.push(plural(posts, WORD.post));
		if (comments) parts.push(plural(comments, WORD.comment));
		return `Соединение установлено — отправляем ${parts.join(' и ')}`;
	}
</script>

{#if notice?.kind === 'offline'}
	<div class="conn" role="status">
		<Icon name="cloud" size="sm" />
		<span>Нет сети. Новое уйдёт само, когда связь появится</span>
	</div>
{:else if notice?.kind === 'down'}
	<div class="conn" role="status">
		<Icon name="clock" size="sm" />
		<span>Сервер не отвечает — доступно сохранённое</span>
	</div>
{:else if notice?.kind === 'sending'}
	<div class="conn sending" role="status">
		<div>{sendingText(notice.posts, notice.comments)}</div>
		<div class="conn-bar"><i style:width="{Math.round(notice.progress * 100)}%"></i></div>
	</div>
{/if}

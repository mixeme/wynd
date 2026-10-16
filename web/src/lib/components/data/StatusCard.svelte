<script lang="ts">
	// Карточка «что происходит» (6.26, 6.31): заголовок и пояснение, без
	// действия. Что делать дальше — под ней, кнопкой или тихой ссылкой.
	let { title, text, class: className = '' }: { title: string; text: string; class?: string } =
		$props();
</script>

<div class="status-card {className}" role="status">
	<div class="status-card-title">{title}</div>
	<div class="status-card-text">{text}</div>
</div>

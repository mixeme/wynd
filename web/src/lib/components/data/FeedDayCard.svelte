<script lang="ts">
	import Icon from '$ui/Icon.svelte';

	// Открытка дня в ленте (3.15): день, каким он стал после записи журнала.
	// Названия нет — на его месте дата; обложку не выбирали — снимка нет.
	let {
		date,
		title,
		coverUrl,
		hasCover = false,
		video = false,
		caption,
		onclick,
		class: className = '',
		style = ''
	}: {
		date: string;
		title?: string;
		coverUrl?: string;
		/** Обложка выбрана — место под снимок есть сразу, картинка доедет. */
		hasCover?: boolean;
		video?: boolean;
		/** Кто, что и когда: «Мама назвала день · вчера, 21:44». */
		caption: string;
		onclick: () => void;
		class?: string;
		style?: string;
	} = $props();
</script>

<button type="button" class="dayc {className}" {style} {onclick}>
	{#if hasCover}
		<span class="pic" style={!coverUrl ? 'background:var(--tint)' : undefined}>
			{#if coverUrl}
				<img src={coverUrl} alt="" />
			{/if}
			{#if video}
				<Icon name="play" class="vid-mark" />
			{/if}
		</span>
	{/if}
	<span class="dayc-tx">
		{#if title}
			<span class="dayc-date">{date}</span>
		{/if}
		<span class="dayc-name">{title || date}</span>
		<span class="dayc-who">{caption}</span>
	</span>
</button>

<script lang="ts">
	import Icon from '$ui/Icon.svelte';
	import SettingsRow from '$ui/data/SettingsRow.svelte';

	let {
		cover,
		title,
		subtitle,
		class: className = '',
		style = ''
	}: {
		cover: string;
		title: string;
		subtitle: string;
		class?: string;
		style?: string;
	} = $props();
</script>

<div class="pic {cover} {className}" {style} style:aspect-ratio="16/9" style:border-radius="0" style:border="0" style:border-bottom="1px solid var(--line)">
	<span class="tagr">обложка дня</span>
	<span class="cnt">сменить</span>
</div>
<SettingsRow {title} {subtitle} />

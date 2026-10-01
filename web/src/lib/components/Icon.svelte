<script lang="ts">
	export type IconName =
		| 'search'
		| 'gear'
		| 'back'
		| 'plus'
		| 'send'
		| 'chev'
		| 'chevr'
		| 'loc'
		| 'play'
		| 'pause'
		| 'heart'
		| 'laugh'
		| 'surprise'
		| 'anger'
		| 'photo'
		| 'file'
		| 'x'
		| 'check'
		| 'filter'
		| 'bell'
		| 'key'
		| 'dots'
		| 'download'
		| 'edit'
		| 'trash'
		| 'clock'
		| 'day'
		| 'cloud'
		| 'out';

	type IconSize = 'md' | 'sm' | 'xs';

	let {
		name,
		size = 'md',
		class: className = '',
		style = ''
	}: {
		name: IconName;
		size?: IconSize;
		class?: string;
		style?: string;
	} = $props();

	const sizeClass = $derived(size === 'sm' ? 'ic sm' : size === 'xs' ? 'ic xs' : 'ic');
</script>

<svg class="{sizeClass} {className}" viewBox="0 0 24 24" {style} aria-hidden="true">
	<use href="/icons.svg#i-{name}" />
</svg>

<script lang="ts" module>
	import sprite from '../../../static/icons.svg?raw';

	// Набор значков — в самой странице, а не ссылкой на /icons.svg: значок,
	// который появился уже без сети (часы у «в очереди», кнопка записи, правка
	// у новой реплики), Firefox на Android не дорисовывал — внешний файл для
	// него не подтягивался, а внутренняя ссылка запроса не делает. Кладётся
	// один раз, при первом значке; так и в клиенте, и в окне Мастерской.
	function ensureSprite(): void {
		if (typeof document === 'undefined' || document.getElementById('wynd-icons')) return;
		const holder = document.createElement('div');
		holder.id = 'wynd-icons';
		holder.hidden = true;
		holder.innerHTML = sprite;
		document.body.prepend(holder);
	}
</script>

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
		| 'out'
		| 'music'
		| 'rec'
		| 'mic'
		| 'video'
		| 'flip'
		| 'stop';

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

	ensureSprite();

	const sizeClass = $derived(size === 'sm' ? 'ic sm' : size === 'xs' ? 'ic xs' : 'ic');
</script>

<svg class="{sizeClass} {className}" viewBox="0 0 24 24" {style} aria-hidden="true">
	<use href="#i-{name}" />
</svg>

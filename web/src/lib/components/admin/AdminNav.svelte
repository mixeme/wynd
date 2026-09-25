<script module lang="ts">
	export const ADMIN_NAV = ['Проверка', 'Общие', 'Доступ', 'Люди', 'Хранилище', 'Сжатие', 'Оплата'] as const;
	export type AdminNavItem = (typeof ADMIN_NAV)[number];
	export const ADMIN_HREF: Record<AdminNavItem, string> = {
		Хранилище: '/admin',
		Проверка: '/admin/check',
		Общие: '/admin/general',
		Сжатие: '/admin/compress',
		Доступ: '/admin/access',
		Люди: '/admin/people',
		Оплата: '/admin/pay'
	};
</script>

<script lang="ts">
	import { goto } from '$app/navigation';

	let {
		active = 'Хранилище',
		links = false,
		class: className = '',
		style = ''
	}: {
		active?: AdminNavItem;
		links?: boolean;
		class?: string;
		style?: string;
	} = $props();
</script>

<span class="admnav {className}" style="margin-left:20px;{style}">
	{#each ADMIN_NAV as item (item)}
		{#if links}
			<button
				type="button"
				class:on={active === item}
				onclick={() => goto(ADMIN_HREF[item])}
			>
				{item}
			</button>
		{:else}
			<span class:on={active === item}>{item}</span>
		{/if}
	{/each}
</span>

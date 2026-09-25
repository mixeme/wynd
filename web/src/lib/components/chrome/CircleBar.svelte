<script module lang="ts">
	export const CIRCLE_TABS = ['Хронология', 'Дни', 'Сетка', 'Карта'] as const;
	export type CircleTab = (typeof CIRCLE_TABS)[number];
</script>

<script lang="ts">
	import { goto } from '$app/navigation';
	import IconButton from '$ui/forms/IconButton.svelte';
	import Avatar from '$ui/data/Avatar.svelte';
	import { Tabs } from 'bits-ui';

	let {
		title,
		identity,
		avatar,
		avatarSrc,
		tabs = true,
		active = $bindable<CircleTab>('Хронология'),
		circleId,
		onback
	}: {
		title: string;
		identity?: string;
		avatar?: string;
		avatarSrc?: string;
		tabs?: boolean;
		active?: CircleTab;
		circleId?: string;
		onback?: () => void;
	} = $props();

	const tabPaths: Record<CircleTab, string> = {
		Хронология: '',
		Дни: '/days',
		Сетка: '/grid',
		Карта: '/map'
	};

	function onTabChange(tab: string) {
		const t = tab as CircleTab;
		active = t;
		if (circleId) {
			const suffix = tabPaths[t];
			goto(suffix ? `/circles/${circleId}${suffix}` : `/circles/${circleId}`);
		}
	}
</script>

<div class="cbar">
	<div class="top">
		{#if onback}
			<IconButton name="back" label="Назад" onclick={() => onback()} />
		{/if}
		<span class="t">{title}</span>
		{#if identity}
			{#if circleId && avatar}
				<button type="button" class="idn" onclick={() => goto(`/circles/${circleId}/settings`)}>
					{identity}<Avatar
						initial={avatar}
						src={avatarSrc}
						style="width:30px;height:30px;font-size:12.5px;border:1px solid rgba(255,255,255,.55)"
					/>
				</button>
			{:else}
				<span class="idn">{identity}{#if avatar}<Avatar
							initial={avatar}
							src={avatarSrc}
							style="width:30px;height:30px;font-size:12.5px;border:1px solid rgba(255,255,255,.55)"
						/>{/if}</span>
			{/if}
		{/if}
	</div>
	{#if tabs}
		<Tabs.Root value={active} onValueChange={(v) => v && onTabChange(v)}>
			<Tabs.List class="tabs">
				{#each CIRCLE_TABS as tab (tab)}
					<Tabs.Trigger value={tab}>
						{#snippet child({ props })}
							<span {...props} class:on={active === tab}>{tab}</span>
						{/snippet}
					</Tabs.Trigger>
				{/each}
			</Tabs.List>
		</Tabs.Root>
	{:else}
		<div style="height:12px"></div>
	{/if}
</div>

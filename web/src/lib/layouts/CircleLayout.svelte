<script lang="ts">
	import { getContext } from 'svelte';
	import { goto } from '$app/navigation';
	import CircleBar, {
		CIRCLE_TAB_PATHS,
		visibleCircleTabs,
		type CircleTab
	} from '$ui/chrome/CircleBar.svelte';
	import {
		SWIPE,
		swipeEnd,
		swipeIdle,
		swipeMove,
		swipeOffset,
		swipeStart
	} from '$lib/gestures/tabSwipe';
	import CommentBar from '$ui/overlays/CommentBar.svelte';
	import PhoneFrame from '$ui/chrome/PhoneFrame.svelte';
	import StatusBar from '$ui/chrome/StatusBar.svelte';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import type { CircleColor } from '$lib/theme/colors';
	import type { Snippet } from 'svelte';

	const circleCtx = getContext<CircleContext | undefined>(CIRCLE_CTX);

	let {
		color,
		dark = false,
		app = false,
		height,
		class: className = '',
		title,
		subtitle,
		identity,
		avatar,
		avatarSrc,
		tabs = true,
		active = $bindable<CircleTab>('Хронология'),
		commentBar = true,
		commentPlaceholder,
		commentDraft = $bindable(''),
		commentMembers = [],
		commentBusy = false,
		circleId: circleIdProp,
		onback,
		onsearch,
		searchPlaceholder,
		searchQuery = $bindable(''),
		onCommentSend,
		onCommentCompose,
		onCommentPhotos,
		identitySettingsLink = true,
		children
	}: {
		color?: CircleColor;
		dark?: boolean;
		app?: boolean;
		height?: string;
		class?: string;
		title: string;
		subtitle?: string;
		identity?: string;
		avatar?: string;
		avatarSrc?: string;
		tabs?: boolean;
		active?: CircleTab;
		commentBar?: boolean;
		commentPlaceholder?: string;
		commentDraft?: string;
		commentMembers?: { identity_id: string; name: string }[];
		commentBusy?: boolean;
		circleId?: string;
		onback?: () => void;
		onsearch?: () => void;
		searchPlaceholder?: string;
		searchQuery?: string;
		onCommentSend?: () => void;
		onCommentCompose?: () => void;
		onCommentPhotos?: (files: File[]) => void;
		identitySettingsLink?: boolean;
		children: Snippet;
	} = $props();

	const circleId = $derived(circleIdProp ?? circleCtx?.circleId);

	// Свайп по вкладкам (план 46, C9): содержимое идёт за пальцем, дальше
	// трети ширины — соседняя вкладка. На «Карте» — только от края.
	let bodyEl: HTMLDivElement | undefined = $state();
	let swipe = $state(swipeIdle());
	let leaving = $state(0);
	const order = $derived(visibleCircleTabs(circleCtx?.hasOthers ?? false, active));
	const tabIndex = $derived(order.indexOf(active));
	const swipeOn = $derived(app && tabs && Boolean(circleId) && tabIndex >= 0);
	const hasPrev = $derived(tabIndex > 0);
	const hasNext = $derived(tabIndex >= 0 && tabIndex < order.length - 1);
	const offset = $derived(leaving || swipeOffset(swipe, hasPrev, hasNext));
	const previewTab = $derived.by(() => {
		if (swipe.phase !== 'horizontal' || Math.abs(swipe.dx) < swipe.width * SWIPE.commit) return undefined;
		const next = swipe.dx < 0 ? tabIndex + 1 : tabIndex - 1;
		return order[next];
	});

	function onSwipeStart(e: TouchEvent) {
		if (!swipeOn || !bodyEl || e.touches.length !== 1 || leaving) return;
		const target = e.target as Element | null;
		const onMap = active === 'Карта';
		if (target?.closest('input, textarea, [contenteditable], .lightbox, [data-no-tab-swipe]')) return;
		if (!onMap && target?.closest('.leaflet-container')) return;
		const t = e.touches[0];
		const rect = bodyEl.getBoundingClientRect();
		swipe = swipeStart(t.clientX - rect.left, t.clientY, rect.width, onMap);
	}

	function onSwipeMove(e: TouchEvent) {
		if (swipe.phase === 'idle' || swipe.phase === 'ignored' || !bodyEl) return;
		const t = e.touches[0];
		swipe = swipeMove(swipe, t.clientX - bodyEl.getBoundingClientRect().left, t.clientY);
	}

	function onSwipeEnd() {
		const dir = swipeEnd(swipe, hasPrev, hasNext);
		const width = swipe.width;
		swipe = swipeIdle();
		if (!dir || !circleId) return;
		const tab = order[tabIndex + dir];
		leaving = dir > 0 ? -width : width;
		setTimeout(() => {
			const suffix = CIRCLE_TAB_PATHS[tab];
			void goto(suffix ? `/circles/${circleId}${suffix}` : `/circles/${circleId}`).finally(() => {
				leaving = 0;
			});
		}, 160);
	}
</script>

<PhoneFrame {color} {dark} {app} {height} class={className}>
	{#if !app}
		<StatusBar />
	{/if}
	<CircleBar
		{title}
		{subtitle}
		{identity}
		{avatar}
		{avatarSrc}
		{tabs}
		{circleId}
		{onback}
		{onsearch}
		{searchPlaceholder}
		bind:searchQuery
		bind:active
		{identitySettingsLink}
		responsesTab={circleCtx?.hasOthers ?? false}
		{previewTab}
		responsesUnread={circleCtx?.responsesUnread ?? 0}
	/>
	<div
		class="circle-body"
		class:tab-swipe={swipeOn && active !== 'Карта'}
		bind:this={bodyEl}
		style:transform={offset ? `translateX(${offset}px)` : undefined}
		style:transition={swipe.phase === 'horizontal' ? 'none' : 'transform .16s ease-out'}
		ontouchstart={onSwipeStart}
		ontouchmove={onSwipeMove}
		ontouchend={onSwipeEnd}
		ontouchcancel={onSwipeEnd}
		role="presentation"
	>
		{@render children()}
	</div>
	{#if commentBar && (onCommentCompose || onCommentSend)}
		<CommentBar
			placeholder={commentPlaceholder}
			bind:value={commentDraft}
			members={commentMembers}
			busy={commentBusy}
			onsend={onCommentSend}
			oncompose={onCommentCompose}
			onphotos={onCommentPhotos}
		/>
	{/if}
</PhoneFrame>

<script lang="ts">
	import { getContext } from 'svelte';
	import CircleBar, { type CircleTab } from '$ui/chrome/CircleBar.svelte';
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
	/>
	<div class="circle-body">
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

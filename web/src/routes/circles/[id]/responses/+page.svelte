<script lang="ts">
	import { goto } from '$app/navigation';
	import { getContext, onMount } from 'svelte';
	import Button from '$ui/forms/Button.svelte';
	import CommentRow from '$ui/data/CommentRow.svelte';
	import EventDivider from '$ui/data/EventDivider.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Icon from '$ui/Icon.svelte';
	import Loading from '$ui/Loading.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { isAccessError } from '$lib/api/client';
	import {
		formatClock,
		formatDayLabel,
		formatEntryDate,
		localDayKey
	} from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { authorInitial, reactionIconName } from '$lib/journal/present';
	import {
		fetchResponses,
		groupResponses,
		markResponsesRead,
		responseHref,
		responseKindLabel,
		responsesDividerIndex,
		type ResponseItem,
		type ResponsePostRef,
		type ResponseRow
	} from '$lib/journal/responses';
	import { resolveMediaUrls } from '$lib/media/batch';
	import { registerRefetch } from '$lib/sync/sync';

	// «Отклики» (3.13): комментарии и реакции — и у старых записей, которые
	// в ленте далеко внизу. Свежее сверху, черта
	// «выше — новое» стоит там, где остановились в прошлый раз.

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let items = $state<ResponseItem[]>([]);
	let posts = $state<Record<string, ResponsePostRef>>({});
	let hasMore = $state(false);
	let loading = $state(true);
	let loadingMore = $state(false);
	let error = $state('');
	let mediaUrls = $state<Record<string, string>>({});
	// Отметка на момент входа: черта не уезжает, пока экран открыт.
	let readSeq = -1;

	const rows = $derived(groupResponses(items, Math.max(readSeq, 0)));
	const dividerAt = $derived(readSeq < 0 ? null : responsesDividerIndex(rows));

	async function resolveMedia(list: ResponseItem[], refs: Record<string, ResponsePostRef>) {
		const ids = new Set<string>();
		for (const it of list) {
			if (it.actor_avatar_blob_id) ids.add(it.actor_avatar_blob_id);
			const ref = it.post_id ? refs[it.post_id] : undefined;
			if (ref?.cover_blob_id) ids.add(ref.cover_blob_id);
		}
		const todo = [...ids].filter((id) => !mediaUrls[id]);
		if (!todo.length) return;
		const next = { ...mediaUrls };
		await resolveMediaUrls(circle.origin, todo, (blobId, url) => {
			next[blobId] = url;
			mediaUrls = { ...next };
		});
	}

	async function load() {
		error = '';
		try {
			const page = await fetchResponses(circle.origin, circle.circleId);
			if (readSeq < 0) readSeq = page.read_seq;
			items = page.items;
			posts = page.posts;
			hasMore = page.has_more;
			void resolveMedia(page.items, page.posts);
			const top = page.items[0]?.seq ?? 0;
			if (top > page.read_seq) {
				await markResponsesRead(circle.origin, circle.circleId, top);
			}
			circle.responsesUnread = 0;
		} catch (err) {
			error = isAccessError(err) ? 'Нет доступа' : 'Не удалось загрузить отклики';
		} finally {
			loading = false;
		}
	}

	async function loadMore() {
		const last = items[items.length - 1];
		if (!last || loadingMore) return;
		loadingMore = true;
		try {
			const page = await fetchResponses(circle.origin, circle.circleId, last.seq);
			items = [...items, ...page.items];
			posts = { ...posts, ...page.posts };
			hasMore = page.has_more;
			void resolveMedia(page.items, page.posts);
		} catch {
			error = 'Не удалось загрузить отклики';
		} finally {
			loadingMore = false;
		}
	}

	onMount(() => {
		void load();
		return registerRefetch({
			origin: circle.origin,
			circleId: circle.circleId,
			kinds: ['feed'],
			refetch: load
		});
	});

	function dayBreak(i: number): string | null {
		const at = rows[i].lead.at;
		if (i > 0 && localDayKey(rows[i - 1].lead.at) === localDayKey(at)) return null;
		return formatDayLabel(at);
	}

	function names(row: ResponseRow): string {
		return row.names.join(', ');
	}

	function postLabel(ref: ResponsePostRef): string {
		return ref.author_identity_id === circle.identityId ? 'ваша запись' : ref.author_name;
	}

	function open(row: ResponseRow, e: MouseEvent) {
		e.preventDefault();
		goto(responseHref(circle.circleId, row.lead));
	}
</script>

<CircleLayout
	app
	color={circle.color}
	title={circle.name}
	identity={circle.identityName}
	avatar={circle.identityInitial}
	avatarSrc={circle.avatarUrl}
	circleId={circle.circleId}
	identitySettingsLink={circle.canWrite}
	active="Отклики"
	commentBar={false}
	onback={() => goto('/circles')}
>
	{#if loading}
		<Loading />
	{:else if error && !items.length}
		<Hint class="gutter-24">{error}</Hint>
	{:else if !items.length}
		<div class="h1s ctr mt-48">Откликов пока нет</div>
		<Hint class="ctr hint-inset">
			Здесь соберутся комментарии и реакции — и к тем записям, что в ленте уже далеко внизу.
		</Hint>
	{:else}
		<div class="resp-list">
			{#each rows as row, i (row.key)}
				{@const day = dayBreak(i)}
				{#if dividerAt === i}
					<EventDivider variant="unread" text="выше — новое" />
				{/if}
				{#if day}
					<EventDivider text={day} />
				{/if}
				{@const ref = row.lead.post_id ? posts[row.lead.post_id] : undefined}
				<a class="resp" href={responseHref(circle.circleId, row.lead)} onclick={(e) => open(row, e)}>
					<CommentRow
						initial={authorInitial(row.lead.actor_name)}
						name={names(row)}
						color={circle.colorHex}
						src={row.lead.actor_avatar_blob_id ? mediaUrls[row.lead.actor_avatar_blob_id] : undefined}
					>
						{#snippet time()}
							{#if row.kind === 'reaction' && row.emoji}<Icon
									name={reactionIconName(row.emoji)}
									size="xs"
									class="resp-rx"
								/>{/if}{responseKindLabel(row.kind)} · {formatClock(row.lead.at)}
						{/snippet}
						{#if row.kind === 'comment' && row.lead.body}
							<div class="pre">{row.lead.body}</div>
						{/if}
						{#if ref}
							<div class="resp-ref">
								{#if ref.cover_blob_id}
									<span class="pic resp-pic"
										>{#if mediaUrls[ref.cover_blob_id]}<img src={mediaUrls[ref.cover_blob_id]} alt="" />{/if}</span
									>
								{/if}
								<span class="resp-line"
									><b>{postLabel(ref)}</b> · {formatEntryDate(ref.entry_date)}{#if ref.excerpt}{' · '}{ref.excerpt}{/if}</span
								>
							</div>
						{/if}
					</CommentRow>
				</a>
			{/each}
			{#if hasMore}
				<Button variant="ghost" class="resp-more" loading={loadingMore} onclick={() => void loadMore()}>
					Показать раньше
				</Button>
			{/if}
		</div>
	{/if}
</CircleLayout>

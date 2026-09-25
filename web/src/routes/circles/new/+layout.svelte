<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import { onMount, setContext } from 'svelte';
	import { fetchCircles } from '$lib/circles/circles';
	import { displayHost } from '$lib/auth/origin';
	import { WORD, plural } from '$lib/format/plural';
	import { loadSessions } from '$lib/session/session.svelte';
	import type { SessionRecord } from '$lib/idb/db';
	import {
		NEW_CIRCLE_CTX,
		type NewCircleContext,
		type NewCircleEditWindow
	} from '$lib/circles/new-circle';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	const form = $state<NewCircleContext>({
		sessions: [],
		circleCounts: {},
		selectedOrigin: '',
		name: '',
		color: 'ochre',
		editWindow: '1h',
		customHours: 24,
		diaryMode: false,
		loading: false,
		error: '',
		ready: false,
		serverSubtitle: () => ''
	});

	setContext(NEW_CIRCLE_CTX, form);

	function serverSubtitle(session: SessionRecord): string {
		const count = form.circleCounts[session.origin] ?? 0;
		const host = displayHost(session.origin);
		const tail = count ? ` · ещё ${plural(count, WORD.yourCircle)}` : '';
		return `${host}${tail}`;
	}

	form.serverSubtitle = serverSubtitle;

	// Свой сервер — пустая строка, поэтому выбор проверяется по наличию
	// в списке, а не на истинность: ?origin= — это выбор своего сервера.
	function applyOriginFromUrl(sessions: SessionRecord[]) {
		const known = (origin: string | null) => sessions.some((s) => s.origin === origin);
		const fromUrl = $page.url.searchParams.get('origin');
		if (known(fromUrl)) {
			form.selectedOrigin = fromUrl ?? '';
		} else if (!known(form.selectedOrigin) && sessions.length) {
			form.selectedOrigin = sessions[0].origin;
		}
	}

	onMount(async () => {
		const sessions = await loadSessions();
		if (!sessions.length) {
			goto('/');
			return;
		}
		form.sessions = sessions;
		const counts: Record<string, number> = {};
		for (const session of sessions) {
			try {
				const list = await fetchCircles(session.origin);
				counts[session.origin] = list.length;
			} catch {
				counts[session.origin] = 0;
			}
		}
		form.circleCounts = counts;
		applyOriginFromUrl(sessions);
		form.ready = true;
	});

	$effect(() => {
		if (!form.ready || !form.sessions.length) return;
		applyOriginFromUrl(form.sessions);
	});
</script>

{@render children()}

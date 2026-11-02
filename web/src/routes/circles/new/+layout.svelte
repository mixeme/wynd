<script lang="ts">
	import { goToLogin } from '$lib/navigation/afterLogin';
	import { page } from '$app/stores';
	import { onMount, setContext } from 'svelte';
	import { fetchCircles } from '$lib/circles/circles';
	import { displayHost } from '$lib/auth/origin';
	import { WORD, plural } from '$lib/format/plural';
	import { loadServerCard, type ServerCardState } from '$lib/instance/serverCard';
	import { loadSessions } from '$lib/session/session.svelte';
	import type { SessionRecord } from '$lib/idb/db';
	import { NEW_CIRCLE_CTX, type NewCircleContext } from '$lib/circles/new-circle';
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();

	const form = $state<NewCircleContext>({
		sessions: [],
		circleCounts: {},
		serverCards: {},
		selectedOrigin: '',
		name: '',
		color: 'ochre',
		editWindow: '1h',
		customHours: 24,
		diaryMode: false,
		loading: false,
		error: '',
		ready: false,
		serverSubtitle: () => '',
		serverDetail: () => undefined
	});

	setContext(NEW_CIRCLE_CTX, form);

	function serverSubtitle(session: SessionRecord): string {
		return displayHost(session.origin);
	}

	form.serverSubtitle = serverSubtitle;
	form.serverDetail = (session, picking = false) => {
		const count = form.circleCounts[session.origin];
		return count === undefined
			? undefined
			: picking
				? plural(count, WORD.circle)
				: count
					? `Ещё ${plural(count, WORD.yourCircle)}`
					: undefined;
	};

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
			goToLogin();
			return;
		}
		form.sessions = sessions;
		const counts: Record<string, number | undefined> = {};
		const cards: Record<string, ServerCardState> = {};
		for (const session of sessions) {
			const [list, card] = await Promise.all([
				fetchCircles(session.origin).catch(() => undefined),
				loadServerCard(session.origin)
			]);
			counts[session.origin] = list?.length;
			cards[session.origin] = card;
		}
		form.circleCounts = counts;
		form.serverCards = cards;
		applyOriginFromUrl(sessions);
		form.ready = true;
	});

	$effect(() => {
		if (!form.ready || !form.sessions.length) return;
		applyOriginFromUrl(form.sessions);
	});
</script>

{@render children()}

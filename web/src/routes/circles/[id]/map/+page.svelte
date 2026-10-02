<script lang="ts">
	import { goUp } from '$lib/navigation/up';
	import { goto } from '$app/navigation';
	import { getContext, onDestroy, onMount } from 'svelte';
	import MapBadge from '$ui/data/MapBadge.svelte';
	import MapPostSheet from '$ui/data/MapPostSheet.svelte';
	import Hint from '$ui/forms/Hint.svelte';
	import Loading from '$ui/Loading.svelte';
	import CircleLayout from '$lib/layouts/CircleLayout.svelte';
	import { isAccessError } from '$lib/api/client';
	import { formatPostTime, pluralPosts } from '$lib/format/time';
	import { CIRCLE_CTX, type CircleContext } from '$lib/journal/context';
	import { loadDays } from '$lib/journal/days';
	import { loadMap } from '$lib/journal/map';
	import { fetchOlderPage, mergePages } from '$lib/journal/pages';
	import type { MapPin } from '$lib/journal/types';
	import { getMediaUrl } from '$lib/media/objectUrl';
	import { registerRefetch } from '$lib/sync/sync';
	import 'leaflet/dist/leaflet.css';
	import 'leaflet.markercluster/dist/MarkerCluster.css';
	import 'leaflet.markercluster/dist/MarkerCluster.Default.css';
	import '$lib/styles/map.css';

	const circle = getContext<CircleContext>(CIRCLE_CTX);

	let mapEl: HTMLDivElement | undefined = $state();
	let pins = $state<MapPin[]>([]);
	let totalPosts = $state(0);
	let loading = $state(true);
	let error = $state('');
	let selected = $state<MapPin | undefined>();
	let selectedUrl = $state('');
	let badge = $state('');

	let map: import('leaflet').Map | undefined;
	let cluster: import('leaflet').MarkerClusterGroup | undefined;
	let leaflet: typeof import('leaflet') | undefined;
	let leafletReady = $state(false);

	// leaflet.markercluster is a UMD plugin without imports: it extends the global L that
	// leaflet always installs, not the ES namespace this module gets from the bundler.
	function clusterGroup(
		L: typeof import('leaflet'),
		options: import('leaflet').MarkerClusterGroupOptions
	): import('leaflet').MarkerClusterGroup {
		const globalL = (globalThis as { L?: typeof import('leaflet') }).L;
		const factory = L.markerClusterGroup ?? globalL?.markerClusterGroup;
		if (!factory) throw new Error('leaflet.markercluster не загрузился');
		return factory(options);
	}

	async function loadData() {
		error = '';
		try {
			const [mapSnap, daysSnap] = await Promise.all([
				loadMap(circle.origin, circle.circleId),
				loadDays(circle.origin, circle.circleId)
			]);
			// Карта показывает всё место круга сразу (C18): старшие порции
			// догружаем разом и ставим одним присваиванием — карта перерисовывается
			// целиком на каждое.
			let older: MapPin[] = [];
			let next = mapSnap.next_before;
			for (let i = 0; next && i < 20; i++) {
				try {
					const page = await fetchOlderPage<{ pins: MapPin[] }>(
						circle.origin,
						`/circles/${circle.circleId}/map`,
						next
					);
					older = [...older, ...page.pins];
					next = page.next_before;
				} catch {
					break;
				}
			}
			pins = mergePages(mapSnap.pins, older, (p) => `${p.post_id}:${p.blob_id}`);
			totalPosts = daysSnap.days.reduce((sum, d) => sum + d.post_count, 0);
			badge = `${pins.length} ${pins.length === 1 ? 'запись' : pins.length < 5 ? 'записи' : 'записей'} с местом из ${pluralPosts(totalPosts)}`;
		} catch (err) {
			error = isAccessError(err) ? 'Нет доступа' : 'Не удалось загрузить карту';
		} finally {
			loading = false;
		}
	}

	async function selectPin(pin: MapPin) {
		selected = pin;
		try {
			selectedUrl = await getMediaUrl(circle.origin, pin.blob_id);
		} catch {
			selectedUrl = '';
		}
	}

	async function renderMap() {
		if (!mapEl || !leaflet) return;
		if (map) {
			map.remove();
			map = undefined;
			cluster = undefined;
		}

		map = leaflet.map(mapEl, { zoomControl: true, attributionControl: true });
		leaflet
			// Без {s}: поддомены a/b/c сняты с осени 2023, адрес — tile.openstreetmap.org.
			// Атрибуция обязана быть ссылкой на условия (LIC-4).
			.tileLayer('https://tile.openstreetmap.org/{z}/{x}/{y}.png', {
				maxZoom: 19,
				attribution:
					'© <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
			})
			.addTo(map);

		cluster = clusterGroup(leaflet, {
			showCoverageOnHover: false,
			maxClusterRadius: 50
		});

		const bounds: import('leaflet').LatLngExpression[] = [];
		for (const pin of pins) {
			const latlng: import('leaflet').LatLngExpression = [pin.geo_lat, pin.geo_lng];
			bounds.push(latlng);
			let icon: import('leaflet').DivIcon;
			try {
				const url = await getMediaUrl(circle.origin, pin.blob_id);
				icon = leaflet.divIcon({
					className: '',
					html: `<div class="map-pin-thumb"><img src="${url}" alt="" /></div>`,
					iconSize: [44, 44],
					iconAnchor: [22, 22]
				});
			} catch {
				icon = leaflet.divIcon({
					className: '',
					html: '<div class="map-pin-thumb"></div>',
					iconSize: [44, 44],
					iconAnchor: [22, 22]
				});
			}
			const marker = leaflet.marker(latlng, { icon });
			marker.on('click', () => {
				void selectPin(pin);
			});
			cluster.addLayer(marker);
		}

		map.addLayer(cluster);

		if (bounds.length === 0) {
			map.setView([20, 0], 2);
		} else if (bounds.length === 1) {
			map.setView(bounds[0], 15);
		} else {
			map.fitBounds(leaflet.latLngBounds(bounds), { padding: [40, 40] });
		}
	}

	// The container lives in the {:else} branch, so it exists only once loading is done:
	// render from an effect that waits for the element, leaflet and the pins together.
	$effect(() => {
		const el = mapEl;
		const ready = leafletReady;
		void pins;
		if (!el || !ready) return;
		renderMap().catch((err) => {
			console.error('map render', err);
			error = 'Не удалось загрузить карту';
		});
	});

	onMount(() => {
		void import('leaflet').then(async (L) => {
			leaflet = L;
			await import('leaflet.markercluster');
			leafletReady = true;
			void loadData();
		});
		const unsub = registerRefetch({
			origin: circle.origin,
			circleId: circle.circleId,
			kinds: ['map', 'days'],
			refetch: loadData
		});
		return unsub;
	});

	onDestroy(() => {
		map?.remove();
	});

	function goBack() {
		goUp('/circles');
	}

	function openSelected() {
		if (selected) goto(`/circles/${circle.circleId}/posts/${selected.post_id}`);
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
	active="Карта"
	commentBar={false}
	onback={goBack}
>
	{#if loading}
		<Loading />
	{:else if error && !pins.length}
		<Hint class="gutter-24">{error}</Hint>
	{:else}
		<div class="map-wrap">
			{#if badge}
				<MapBadge {badge} />
			{/if}
			<div bind:this={mapEl} style="flex:1;min-height:420px"></div>
			{#if selected}
				<MapPostSheet
					thumbUrl={selectedUrl || undefined}
					author={selected.author_name}
					time={formatPostTime(selected.created_at, selected.entry_date)}
					body={selected.body}
					onclick={openSelected}
				/>
			{/if}
		</div>
	{/if}
</CircleLayout>

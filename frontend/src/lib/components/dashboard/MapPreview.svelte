<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { getPhotosMap, type PhotoMapItem } from '$lib/api';
	import { loadLeaflet, createTealMarkerIcon, createOsmTileLayer } from '$lib/map';
	import { Card, Badge } from '$lib/components/ui';
	import { IconMapPin, IconArrowRight, IconMap } from '@tabler/icons-svelte';
	import type * as LeafletType from 'leaflet';

	let leafletMap: LeafletType.Map | null = null;
	let photosWithGps = $state<PhotoMapItem[]>([]);
	let isLoading = $state(true);

	onMount(async () => {
		try {
			photosWithGps = await getPhotosMap();
		} catch {
			// Bei Fehlern Widget still verbergen
			photosWithGps = [];
		} finally {
			isLoading = false;
		}
	});

	onDestroy(() => {
		if (leafletMap) {
			leafletMap.remove();
			leafletMap = null;
		}
	});

	// Svelte-Aktion: Wird garantiert ausgefuehrt, sobald der Knoten im DOM montiert ist
	function initMiniMap(node: HTMLElement) {
		let destroyed = false;

		(async () => {
			try {
				const L = await loadLeaflet();
				if (destroyed) return;

				leafletMap = L.map(node, {
					zoomControl: false,
					attributionControl: false,
					dragging: false,
					touchZoom: false,
					scrollWheelZoom: false,
					doubleClickZoom: false,
					boxZoom: false,
					keyboard: false
				});

				createOsmTileLayer(L).addTo(leafletMap);

				const tealIcon = createTealMarkerIcon(L);
				const group = L.featureGroup();

				for (const p of photosWithGps) {
					const m = L.marker([p.gps_lat, p.gps_lon], { icon: tealIcon });
					group.addLayer(m);
				}

				leafletMap.addLayer(group);

				if (photosWithGps.length > 0) {
					leafletMap.fitBounds(group.getBounds().pad(0.2));
				} else {
					leafletMap.setView([50, 10], 4);
				}

				// Verhindert graue Kacheln durch verzoegerte Groessenberechnung
				setTimeout(() => {
					if (!destroyed && leafletMap) {
						leafletMap.invalidateSize();
					}
				}, 150);
			} catch (err) {
				console.error('Fehler beim Initialisieren der Mini-Karte:', err);
			}
		})();

		return {
			destroy() {
				destroyed = true;
				if (leafletMap) {
					leafletMap.remove();
					leafletMap = null;
				}
			}
		};
	}
</script>

{#if !isLoading && photosWithGps.length > 0}
	<Card class="p-5">
		<!-- Header mit Accent-Akzentbalken -->
		<div class="flex items-center justify-between pb-3 border-b border-border-light dark:border-border-dark">
			<div class="flex items-center gap-2.5">
				<div class="w-0.5 h-4 rounded-full bg-accent shrink-0"></div>
				<div class="p-1.5 rounded-lg bg-accent/15 text-accent">
					<IconMap class="w-4 h-4" />
				</div>
				<h3 class="font-semibold text-sm text-text-light dark:text-text-dark">
					Fotokarte
				</h3>
			</div>
			<Badge variant="neutral" size="sm">
				<span class="tabular-nums">{photosWithGps.length}</span>
			</Badge>
		</div>

		<!-- Mini-Karte (max 400x300, hier 180px Hoehe) mit Overlay-Link -->
		<div class="mt-4 relative group">
			<a
				href="/photos/map"
				class="block h-[180px] w-full rounded-xl overflow-hidden border border-border-light dark:border-border-dark relative shadow-xs"
				title="Große Fotokarte öffnen"
			>
				<div use:initMiniMap class="w-full h-full pointer-events-none"></div>
				<div class="absolute inset-0 bg-transparent group-hover:bg-black/10 transition-colors"></div>
			</a>
			<span class="block text-3xs text-muted-light dark:text-muted-dark mt-1 text-right">
				&copy; OpenStreetMap contributors
			</span>
		</div>

		<div class="mt-3 pt-3 border-t border-border-light dark:border-border-dark">
			<a
				href="/photos/map"
				class="text-xs font-medium text-primary dark:text-primary-light hover:underline inline-flex items-center justify-between w-full"
			>
				<span>Auf Vollbildkarte anzeigen</span>
				<IconArrowRight class="w-3.5 h-3.5" />
			</a>
		</div>
	</Card>
{/if}

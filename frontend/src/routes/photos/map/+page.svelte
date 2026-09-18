<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/state';
	import { getPhotosMap, type PhotoMapItem } from '$lib/api';
	import { formatDate } from '$lib/files';
	import { loadLeaflet, createTealMarkerIcon, createOsmTileLayer } from '$lib/map';
	import { Button } from '$lib/components/ui';
	import {
		IconArrowLeft,
		IconMapPin,
		IconInfoCircle,
		IconPhoto
	} from '@tabler/icons-svelte';
	import type * as LeafletType from 'leaflet';

	let mapContainer = $state<HTMLDivElement>();
	let leafletMap: LeafletType.Map | null = null;
	let photos = $state<PhotoMapItem[]>([]);
	let isLoading = $state(true);
	let error = $state<string | null>(null);

	// Ziel-Foto aus URL ?photo=<id>
	let targetPhotoId = $derived(page.url.searchParams.get('photo'));

	onMount(async () => {
		try {
			// 1. Fotos mit GPS abrufen
			photos = await getPhotosMap();

			// 2. Leaflet und MarkerCluster dynamisch laden
			const L = await loadLeaflet();

			if (!mapContainer) return;

			// Karte initialisieren
			leafletMap = L.map(mapContainer, {
				zoomControl: false,
				attributionControl: true
			});

			L.control.zoom({ position: 'bottomright' }).addTo(leafletMap);
			createOsmTileLayer(L).addTo(leafletMap);

			const tealIcon = createTealMarkerIcon(L);

			// Marker-Cluster initialisieren: Cluster bei Zoom < 10, Einzelmarker ab >= 12
			// @ts-ignore
			const clusterGroup = (L as any).markerClusterGroup({
				disableClusteringAtZoom: 12,
				maxClusterRadius: 45,
				showCoverageOnHover: false
			});

			const markerMap = new Map<string, LeafletType.Marker>();

			for (const item of photos) {
				const marker = L.marker([item.gps_lat, item.gps_lon], { icon: tealIcon });

				const popupContent = `
					<div class="p-1 min-w-[170px] text-neutral-900 font-sans">
						<img
							src="/api/v1/files/${item.id}/thumbnail"
							alt="${item.filename}"
							class="w-full h-24 object-cover rounded-md mb-2 bg-neutral-200"
							onerror="this.style.display='none'"
						/>
						<div class="font-semibold text-xs truncate">${item.filename}</div>
						${item.location_name ? `<div class="text-[11px] text-neutral-600 mt-0.5 truncate">${item.location_name}</div>` : ''}
						${item.taken_at ? `<div class="text-[10px] text-neutral-400 mt-1">${formatDate(item.taken_at)}</div>` : ''}
					</div>
				`;

				marker.bindPopup(popupContent, { minWidth: 170 });
				clusterGroup.addLayer(marker);
				markerMap.set(item.id, marker);
			}

			leafletMap.addLayer(clusterGroup);

			// Fokus-Logik: Falls photo=<id> in URL, darauf zentrieren
			if (targetPhotoId && markerMap.has(targetPhotoId)) {
				const targetItem = photos.find((p) => p.id === targetPhotoId);
				if (targetItem) {
					leafletMap.setView([targetItem.gps_lat, targetItem.gps_lon], 14);
					const m = markerMap.get(targetPhotoId);
					if (m) {
						// Bei MarkerCluster zugehoerigen Marker oeffnen
						clusterGroup.zoomToShowLayer(m, () => {
							m.openPopup();
						});
					}
				}
			} else if (photos.length > 0) {
				leafletMap.fitBounds(clusterGroup.getBounds().pad(0.15));
			} else {
				// Standard-Ansicht Europa
				leafletMap.setView([50.1109, 8.6821], 5);
			}
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : 'Karte konnte nicht geladen werden';
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
</script>

<div class="space-y-4">
	<!-- Seiten-Header mit Zurueck-Button -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b border-border-light dark:border-border-dark">
		<div class="flex items-center gap-3">
			<Button href="/photos" variant="secondary" size="sm">
				<IconArrowLeft size={16} stroke={1.75} class="mr-1" />
				<span>Zurück zur Galerie</span>
			</Button>

			<div>
				<h1 class="text-xl sm:text-2xl font-bold tracking-tight text-text-light dark:text-text-dark flex items-center gap-2">
					<IconMapPin size={22} class="text-accent" />
					<span>Fotokarte</span>
				</h1>
			</div>
		</div>

		<div class="flex items-center gap-2 text-xs text-muted-light dark:text-muted-dark">
			<span class="inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-surface-light dark:bg-surface-dark border border-border-light dark:border-border-dark font-medium shadow-xs">
				<IconPhoto size={14} class="text-accent" />
				<span class="tabular-nums font-semibold text-text-light dark:text-text-dark">{photos.length}</span>
				<span>verortete Fotos</span>
			</span>
		</div>
	</div>

	<!-- Karten-Container -->
	<div class="relative w-full h-[calc(100vh-210px)] min-h-[500px] rounded-2xl overflow-hidden border border-border-light dark:border-border-dark shadow-depth card-depth bg-surface-light dark:bg-surface-dark">
		<!-- DSGVO / OSM Hinweis-Banner -->
		<div class="absolute top-3 right-3 z-400 px-3 py-1.5 rounded-xl bg-surface-light/90 dark:bg-surface-dark/90 backdrop-blur-md border border-border-light dark:border-border-dark text-[11px] text-muted-light dark:text-muted-dark flex items-center gap-1.5 shadow-md">
			<IconInfoCircle size={14} class="text-accent shrink-0" />
			<span>Kartenkacheln von OpenStreetMap</span>
		</div>

		<!-- Ladeanzeige -->
		{#if isLoading}
			<div class="absolute inset-0 z-500 bg-surface-light/80 dark:bg-surface-dark/80 backdrop-blur-xs flex flex-col items-center justify-center text-center p-6">
				<div class="w-10 h-10 border-3 border-primary/30 border-t-primary rounded-full animate-spin mb-3"></div>
				<p class="text-sm font-semibold text-text-light dark:text-text-dark">Standorte und Karte werden geladen...</p>
			</div>
		{/if}

		<!-- Fehleranzeige -->
		{#if error}
			<div class="absolute inset-0 z-500 flex items-center justify-center p-6">
				<div class="max-w-md p-5 rounded-xl bg-danger/10 border border-danger/20 text-danger text-center text-xs font-medium">
					{error}
				</div>
			</div>
		{/if}

		<!-- Empty State wenn 0 GPS-Fotos vorhanden -->
		{#if !isLoading && !error && photos.length === 0}
			<div class="absolute inset-0 z-300 flex flex-col items-center justify-center p-6 text-center bg-surface-light/60 dark:bg-surface-dark/60 backdrop-blur-xs">
				<div class="w-12 h-12 rounded-full bg-slate-100 dark:bg-slate-800 flex items-center justify-center text-muted-light dark:text-muted-dark mb-3">
					<IconMapPin size={24} class="opacity-60" />
				</div>
				<p class="text-sm font-semibold text-text-light dark:text-text-dark">Keine Fotos mit Standortdaten vorhanden</p>
				<p class="text-xs text-muted-light dark:text-muted-dark mt-1 max-w-sm">
					Laden Sie Fotos mit EXIF-GPS-Koordinaten hoch, um sie auf der interaktiven Karte anzuzeigen.
				</p>
			</div>
		{/if}

		<!-- Leaflet Container -->
		<div bind:this={mapContainer} class="w-full h-full z-10 select-none"></div>
	</div>
</div>

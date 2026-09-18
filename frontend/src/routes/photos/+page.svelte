<script lang="ts">
	import { onMount } from 'svelte';
	import { listPhotos, type PhotoItem } from '$lib/photos';
	import PhotoGrid from '$lib/components/PhotoGrid.svelte';
	import Lightbox from '$lib/components/Lightbox.svelte';
	import { Button } from '$lib/components/ui';
	import {
		IconUpload,
		IconSearch,
		IconAlbum,
		IconMap,
		IconMapPin,
		IconCurrentLocation
	} from '@tabler/icons-svelte';

	let photos = $state<PhotoItem[]>([]);
	let isLoading = $state(true);
	let error = $state<string | null>(null);

	// Filter-Zustände
	let activeFilter = $state<'all' | 'images' | 'videos'>('all');
	let searchQuery = $state('');
	let selectedYear = $state<string>('all');
	let filterWithLocation = $state<boolean>(false);
	let filterWithGps = $state<boolean>(false);

	let page = $state(1);
	let total = $state(0);
	let limit = $state(100);

	let selectedPhotoIndex = $state<number | null>(null);

	onMount(async () => {
		await loadPhotos();
	});

	async function loadPhotos(resetPage = false) {
		if (resetPage) {
			page = 1;
		}
		isLoading = true;
		error = null;

		try {
			const res = await listPhotos(activeFilter, page, limit, searchQuery);
			photos = res.files || [];
			total = res.total;
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : 'Fehler beim Laden der Fotos';
		} finally {
			isLoading = false;
		}
	}

	function handleFilterChange(filter: 'all' | 'images' | 'videos') {
		if (activeFilter === filter) return;
		activeFilter = filter;
		loadPhotos(true);
	}

	function handleSearchInput(e: Event) {
		const val = (e.target as HTMLInputElement).value;
		searchQuery = val;
		loadPhotos(true);
	}

	// Dynamische Jahre aus allen geladenen Fotos ermitteln
	let availableYears = $derived.by(() => {
		const years = new Set<string>();
		for (const p of photos) {
			const d = p.taken_at || p.created_at;
			if (d) {
				const year = new Date(d).getFullYear();
				if (!isNaN(year)) years.add(String(year));
			}
		}
		return Array.from(years).sort((a, b) => Number(b) - Number(a));
	});

	// Gefilterte Fotos nach Jahr, Ort und GPS
	let filteredPhotos = $derived.by(() => {
		return photos.filter((p) => {
			// 1. Jahr-Filter
			if (selectedYear !== 'all') {
				const d = p.taken_at || p.created_at;
				if (!d) return false;
				const y = String(new Date(d).getFullYear());
				if (y !== selectedYear) return false;
			}

			// 2. Mit Ort (location_name vorhanden)
			if (filterWithLocation) {
				if (!p.location_name || p.location_name.trim() === '') return false;
			}

			// 3. Mit GPS (gps_lat vorhanden)
			if (filterWithGps) {
				if (p.gps_lat === null || p.gps_lat === undefined) return false;
			}

			return true;
		});
	});

	function openLightbox(photo: PhotoItem) {
		const idx = filteredPhotos.findIndex((p) => p.id === photo.id);
		if (idx !== -1) {
			selectedPhotoIndex = idx;
		}
	}

	function closeLightbox() {
		selectedPhotoIndex = null;
	}
</script>

<div class="space-y-6">
	<!-- Seiten-Header mit Aktionen -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b border-border-light dark:border-border-dark">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-text-light dark:text-text-dark">Fotos & Medien</h1>
			<p class="text-xs sm:text-sm text-muted-light dark:text-muted-dark mt-1">
				DSGVO-konforme Galerie mit hierarchischer Timeline, automatischer EXIF-Auslesung und RLS-Schutz.
			</p>
		</div>

		<div class="flex items-center gap-2.5">
			<Button href="/photos/map" variant="secondary" size="sm">
				<IconMap size={16} stroke={1.75} class="mr-1.5 text-accent" />
				<span>Karte</span>
			</Button>

			<Button href="/photos/albums" variant="secondary" size="sm">
				<IconAlbum size={16} stroke={1.75} class="mr-1.5" />
				<span>Alben</span>
			</Button>

			<Button href="/files/upload" variant="primary" size="sm">
				<IconUpload size={16} stroke={1.75} class="mr-1.5" />
				<span>Fotos hochladen</span>
			</Button>
		</div>
	</div>

	<!-- Filter- und Suchleiste -->
	<div class="flex flex-col lg:flex-row lg:items-center justify-between gap-4">
		<!-- Linke Filtergruppe (Typ-Tabs, Jahr-Dropdown, Toggles) -->
		<div class="flex flex-wrap items-center gap-2.5">
			<!-- Filter Tabs (Medientyp) -->
			<div class="flex items-center gap-1 bg-slate-100 dark:bg-surface-dark p-1 rounded-xl border border-border-light dark:border-border-dark">
				<button
					type="button"
					class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors cursor-pointer {
						activeFilter === 'all'
							? 'bg-primary text-white shadow-xs'
							: 'text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark'
					}"
					onclick={() => handleFilterChange('all')}
				>
					Alle Medien
				</button>
				<button
					type="button"
					class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors cursor-pointer {
						activeFilter === 'images'
							? 'bg-primary text-white shadow-xs'
							: 'text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark'
					}"
					onclick={() => handleFilterChange('images')}
				>
					Nur Fotos
				</button>
				<button
					type="button"
					class="px-3 py-1.5 rounded-lg text-xs font-medium transition-colors cursor-pointer {
						activeFilter === 'videos'
							? 'bg-primary text-white shadow-xs'
							: 'text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark'
					}"
					onclick={() => handleFilterChange('videos')}
				>
					Videos
				</button>
			</div>

			<!-- Jahr-Dropdown -->
			<select
				bind:value={selectedYear}
				class="rounded-xl border border-border-light dark:border-border-dark bg-surface-light dark:bg-surface-dark px-3 py-1.5 text-xs text-text-light dark:text-text-dark focus:outline-hidden focus:border-primary cursor-pointer shadow-xs"
			>
				<option value="all">Alle Jahre</option>
				{#each availableYears as year}
					<option value={year}>{year}</option>
				{/each}
			</select>

			<!-- Toggle: Mit Ort -->
			<button
				type="button"
				onclick={() => (filterWithLocation = !filterWithLocation)}
				class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-medium border transition-colors cursor-pointer shadow-xs {
					filterWithLocation
						? 'bg-accent/15 text-accent border-accent/40 font-semibold'
						: 'border-border-light dark:border-border-dark text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark bg-surface-light dark:bg-surface-dark'
				}"
				title="Nur Fotos mit erkanntem Standort"
			>
				<IconMapPin size={14} stroke={1.75} />
				<span>Mit Ort</span>
			</button>

			<!-- Toggle: Mit GPS -->
			<button
				type="button"
				onclick={() => (filterWithGps = !filterWithGps)}
				class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-medium border transition-colors cursor-pointer shadow-xs {
					filterWithGps
						? 'bg-accent/15 text-accent border-accent/40 font-semibold'
						: 'border-border-light dark:border-border-dark text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark bg-surface-light dark:bg-surface-dark'
				}"
				title="Nur Fotos mit GPS-Koordinaten"
			>
				<IconCurrentLocation size={14} stroke={1.75} />
				<span>Mit GPS</span>
			</button>
		</div>

		<!-- Dateinamensuche -->
		<div class="relative w-full lg:w-64">
			<IconSearch size={16} stroke={1.75} class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-light dark:text-muted-dark pointer-events-none" />
			<input
				type="text"
				bind:value={searchQuery}
				oninput={handleSearchInput}
				placeholder="Fotos suchen..."
				class="w-full rounded-xl border border-border-light dark:border-border-dark bg-surface-light dark:bg-surface-dark px-3.5 py-1.5 pl-9 text-xs text-text-light dark:text-text-dark placeholder-muted-light dark:placeholder-muted-dark focus:outline-hidden focus:border-primary transition-all shadow-xs"
			/>
		</div>
	</div>

	<!-- Hauptinhalt Galerie mit Skeleton bei Ladephasen -->
	{#if isLoading && photos.length === 0}
		<div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-4 gap-3.5">
			{#each Array(8) as _}
				<div class="aspect-square rounded-xl bg-slate-200/70 dark:bg-slate-800/60 animate-skeleton"></div>
			{/each}
		</div>
	{:else if error}
		<div class="rounded-xl border border-danger/20 bg-danger/10 p-4 text-xs font-medium text-danger">
			{error}
		</div>
	{:else}
		<PhotoGrid photos={filteredPhotos} onSelectPhoto={openLightbox} />
	{/if}
</div>

<!-- Lightbox Modal -->
{#if selectedPhotoIndex !== null && filteredPhotos[selectedPhotoIndex]}
	<Lightbox
		photos={filteredPhotos}
		currentIndex={selectedPhotoIndex}
		onClose={closeLightbox}
	/>
{/if}

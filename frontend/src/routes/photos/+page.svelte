<script lang="ts">
	import { onMount } from 'svelte';
	import { listPhotos, type PhotoItem } from '$lib/photos';
	import PhotoGrid from '$lib/components/PhotoGrid.svelte';
	import Lightbox from '$lib/components/Lightbox.svelte';
	import { Button } from '$lib/components/ui';
	import { IconUpload, IconSearch, IconAlbum } from '@tabler/icons-svelte';

	let photos = $state<PhotoItem[]>([]);
	let isLoading = $state(true);
	let error = $state<string | null>(null);

	let activeFilter = $state<'all' | 'images' | 'videos'>('all');
	let searchQuery = $state('');
	let page = $state(1);
	let total = $state(0);
	let limit = $state(60);

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

	function openLightbox(photo: PhotoItem) {
		const idx = photos.findIndex((p) => p.id === photo.id);
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
				DSGVO-konforme Galerie mit automatischer EXIF-Bereinigung und RLS-Schutz.
			</p>
		</div>

		<div class="flex items-center gap-2.5">
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
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
		<!-- Filter Tabs -->
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

		<!-- Dateinamensuche -->
		<div class="relative w-full sm:w-64">
			<IconSearch size={16} stroke={1.75} class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-light dark:text-muted-dark pointer-events-none" />
			<input
				type="text"
				bind:value={searchQuery}
				oninput={handleSearchInput}
				placeholder="Fotos suchen..."
				class="w-full rounded-lg border border-border-light dark:border-border-dark bg-surface-light dark:bg-surface-dark px-3.5 py-1.5 pl-9 text-xs text-text-light dark:text-text-dark placeholder-muted-light dark:placeholder-muted-dark focus:outline-hidden focus:border-primary transition-all"
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
		<PhotoGrid {photos} onSelectPhoto={openLightbox} />
	{/if}
</div>

<!-- Lightbox Modal -->
{#if selectedPhotoIndex !== null}
	<Lightbox
		{photos}
		currentIndex={selectedPhotoIndex}
		onClose={closeLightbox}
	/>
{/if}

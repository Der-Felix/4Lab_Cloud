<script lang="ts">
	import { onMount } from 'svelte';
	import { listTags, createTag, getAlbumFiles, type TagItem, type PhotoItem } from '$lib/photos';
	import PhotoGrid from '$lib/components/PhotoGrid.svelte';
	import Lightbox from '$lib/components/Lightbox.svelte';
	import { Button } from '$lib/components/ui';
	import { ArrowLeft, Plus, FolderClosed, Layers } from '@lucide/svelte';

	let tags = $state<TagItem[]>([]);
	let isLoading = $state(true);
	let error = $state<string | null>(null);

	// Ausgewaehltes Album fuer die Detailansicht
	let selectedTag = $state<TagItem | null>(null);
	let albumPhotos = $state<PhotoItem[]>([]);
	let isLoadingAlbum = $state(false);

	// Modal zum Erstellen eines neuen Albums
	let showCreateModal = $state(false);
	let newAlbumName = $state('');
	let newAlbumColor = $state('');
	let isCreating = $state(false);

	// Lightbox
	let selectedPhotoIndex = $state<number | null>(null);

	onMount(async () => {
		await loadTags();
	});

	async function loadTags() {
		isLoading = true;
		error = null;
		try {
			tags = await listTags();
		} catch (err: unknown) {
			error = err instanceof Error ? err.message : 'Fehler beim Laden der Alben';
		} finally {
			isLoading = false;
		}
	}

	async function openAlbum(tag: TagItem) {
		selectedTag = tag;
		isLoadingAlbum = true;
		try {
			const res = await getAlbumFiles(tag.id);
			albumPhotos = res.files || [];
		} catch (err) {
			console.error('Album konnte nicht geladen werden:', err);
		} finally {
			isLoadingAlbum = false;
		}
	}

	function closeAlbum() {
		selectedTag = null;
		albumPhotos = [];
	}

	async function handleCreateAlbum(e: SubmitEvent) {
		e.preventDefault();
		if (!newAlbumName.trim() || isCreating) return;
		isCreating = true;
		try {
			const created = await createTag(newAlbumName.trim(), newAlbumColor || undefined);
			tags = [...tags, created];
			showCreateModal = false;
			newAlbumName = '';
			newAlbumColor = '';
		} catch (err: unknown) {
			alert(err instanceof Error ? err.message : 'Album konnte nicht erstellt werden');
		} finally {
			isCreating = false;
		}
	}
</script>

<svelte:head>
	<title>Alben - 4labscloud</title>
</svelte:head>

<div class="space-y-6">
	<!-- Wenn ein Album geoeffnet ist: Detailansicht -->
	{#if selectedTag}
		<div class="flex items-center justify-between pb-2 border-b border-border-light dark:border-border-dark">
			<div class="flex items-center space-x-3">
				<Button
					variant="secondary"
					size="sm"
					onclick={closeAlbum}
					title="Zurück zu allen Alben"
				>
					<ArrowLeft class="w-4 h-4" />
				</Button>
				<div>
					<div class="flex items-center gap-2">
						<h1 class="text-2xl font-bold tracking-tight text-text-light dark:text-text-dark">
							{selectedTag.name}
						</h1>
					</div>
					<p class="text-xs text-muted-light dark:text-muted-dark mt-0.5 tabular-nums">
						{albumPhotos.length} {albumPhotos.length === 1 ? 'Foto' : 'Fotos'} in diesem Album
					</p>
				</div>
			</div>
		</div>

		{#if isLoadingAlbum}
			<div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-4 gap-3.5">
				{#each Array(8) as _}
					<div class="aspect-square rounded-xl bg-slate-200/70 dark:bg-slate-800/60 animate-skeleton"></div>
				{/each}
			</div>
		{:else}
			<PhotoGrid photos={albumPhotos} onSelectPhoto={(p) => (selectedPhotoIndex = albumPhotos.findIndex(x => x.id === p.id))} />
		{/if}
	{:else}
		<!-- Alben-Uebersicht -->
		<div class="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 pb-2 border-b border-border-light dark:border-border-dark">
			<div>
				<h1 class="text-2xl font-bold tracking-tight text-text-light dark:text-text-dark">Alben</h1>
				<p class="text-xs sm:text-sm text-muted-light dark:text-muted-dark mt-1">
					Tag-basierte Sammlungen für Fotos und Medien
				</p>
			</div>

			<div class="flex items-center space-x-3">
				<Button href="/photos" variant="secondary" size="sm">
					<ArrowLeft class="w-4 h-4 mr-1.5" />
					<span>Zur Galerie</span>
				</Button>

				<Button
					variant="primary"
					size="sm"
					onclick={() => (showCreateModal = true)}
				>
					<Plus class="w-4 h-4 mr-1.5" />
					<span>Neues Album</span>
				</Button>
			</div>
		</div>

		{#if isLoading}
			<div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
				{#each Array(4) as _}
					<div class="h-32 rounded-2xl bg-slate-200/70 dark:bg-slate-800/60 animate-skeleton"></div>
				{/each}
			</div>
		{:else if error}
			<div class="rounded-xl border border-danger/20 bg-danger/10 p-4 text-xs font-medium text-danger">
				{error}
			</div>
		{:else if tags.length === 0}
			<div class="flex flex-col items-center justify-center py-20 text-center text-muted-light dark:text-muted-dark">
				<Layers class="h-14 w-14 mb-4 opacity-40" />
				<p class="text-base font-medium text-text-light dark:text-text-dark">Noch keine Alben angelegt</p>
				<p class="text-xs mt-1">Erstellen Sie ein Album, um Fotos thematisch zu gruppieren.</p>
				<div class="mt-4">
					<Button
						variant="primary"
						size="sm"
						onclick={() => (showCreateModal = true)}
					>
						<span>Album erstellen</span>
					</Button>
				</div>
			</div>
		{:else}
			<div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-4">
				{#each tags as tag (tag.id)}
					<button
						type="button"
						onclick={() => openAlbum(tag)}
						class="group flex flex-col p-5 rounded-2xl border border-border-light dark:border-border-dark bg-surface-light dark:bg-surface-dark hover:border-primary/50 transition-all text-left cursor-pointer shadow-soft"
					>
						<div class="flex items-center justify-between mb-4">
							<div class="w-10 h-10 rounded-xl flex items-center justify-center bg-primary/10 text-primary dark:text-primary-light">
								<FolderClosed class="w-5 h-5" />
							</div>
							<span class="text-xs font-semibold px-2 py-0.5 rounded-full bg-slate-100 dark:bg-slate-800 text-muted-light dark:text-muted-dark tabular-nums">
								{tag.file_count ?? 0}
							</span>
						</div>
						<h3 class="text-sm font-semibold text-text-light dark:text-text-dark group-hover:text-primary dark:group-hover:text-primary-light transition-colors truncate">
							{tag.name}
						</h3>
						<span class="text-xs text-muted-light dark:text-muted-dark mt-1">
							Klicken zum Öffnen
						</span>
					</button>
				{/each}
			</div>
		{/if}
	{/if}
</div>

<!-- Modal: Neues Album erstellen -->
{#if showCreateModal}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4 backdrop-blur-xs">
		<div class="w-full max-w-md rounded-2xl bg-surface-light dark:bg-surface-dark p-6 shadow-xl border border-border-light dark:border-border-dark">
			<h3 class="text-base font-semibold text-text-light dark:text-text-dark mb-4">Neues Album erstellen</h3>

			<form onsubmit={handleCreateAlbum} class="space-y-4">
				<div>
					<label for="album-name" class="block text-xs font-medium text-text-light dark:text-text-dark mb-1">
						Name des Albums
					</label>
					<input
						id="album-name"
						type="text"
						bind:value={newAlbumName}
						placeholder="z.B. Dokumente 2026"
						class="w-full rounded-lg border border-border-light dark:border-border-dark bg-surface-light dark:bg-bg-dark px-3.5 py-2 text-xs text-text-light dark:text-text-dark placeholder-muted-light dark:placeholder-muted-dark focus:border-primary focus:outline-hidden"
						required
					/>
				</div>

				<div class="flex justify-end gap-2 pt-4">
					<Button
						variant="secondary"
						size="sm"
						onclick={() => (showCreateModal = false)}
					>
						<span>Abbrechen</span>
					</Button>
					<Button
						type="submit"
						variant="primary"
						size="sm"
						loading={isCreating}
						disabled={!newAlbumName.trim()}
					>
						<span>Album anlegen</span>
					</Button>
				</div>
			</form>
		</div>
	</div>
{/if}

<!-- Lightbox Modal im Album -->
{#if selectedPhotoIndex !== null}
	<Lightbox
		photos={albumPhotos}
		currentIndex={selectedPhotoIndex}
		onClose={() => (selectedPhotoIndex = null)}
	/>
{/if}

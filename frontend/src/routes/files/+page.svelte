<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page as pageState } from '$app/state';
	import FileList from '$lib/components/FileList.svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
	import ShareDialog from '$lib/components/ShareDialog.svelte';
	import Uploader from '$lib/components/Uploader.svelte';
	import { Button } from '$lib/components/ui';
	import { addToast } from '$lib/stores';
	import {
		listFiles,
		downloadFile,
		deleteFile,
		renameFile,
		type FileItem
	} from '$lib/files';
	import { uploadCompletedTrigger } from '$lib/uploader';
	import {
		IconFolder,
		IconPhoto,
		IconFileText,
		IconVideo,
		IconMusic,
		IconUpload,
		IconSearch,
		IconRefresh,
		IconAlertCircle,
		IconList,
		IconLayoutGrid,
		IconX
	} from '@tabler/icons-svelte';

	let files = $state<FileItem[]>([]);
	let total = $state(0);
	let page = $state(1);
	let limit = $state(50);
	let sort = $state('date');
	let order = $state<'asc' | 'desc'>('desc');
	let filter = $state('');
	let activeCategory = $state<'all' | 'images' | 'docs' | 'videos' | 'audio'>('all');
	let viewMode = $state<'list' | 'grid'>('list');
	let isLoading = $state(false);
	let errorMessage = $state('');

	// Dialog-Zustaende
	let fileToDelete = $state<FileItem | null>(null);
	let bulkDeleteIds = $state<string[]>([]);
	let showDeleteDialog = $state(false);
	let fileToShare = $state<FileItem | null>(null);
	let showShareDialog = $state(false);
	let showUploadModal = $state(false);

	let searchDebounceTimer: ReturnType<typeof setTimeout> | null = null;

	async function loadFiles() {
		isLoading = true;
		errorMessage = '';
		try {
			let effectiveFilter = filter;
			if (activeCategory === 'images') effectiveFilter = effectiveFilter ? `${effectiveFilter} image` : 'image';
			else if (activeCategory === 'docs') effectiveFilter = effectiveFilter ? `${effectiveFilter} pdf` : 'pdf';
			else if (activeCategory === 'videos') effectiveFilter = effectiveFilter ? `${effectiveFilter} video` : 'video';
			else if (activeCategory === 'audio') effectiveFilter = effectiveFilter ? `${effectiveFilter} audio` : 'audio';

			const res = await listFiles(page, limit, sort, order, effectiveFilter);
			files = res.files;
			total = res.total;
			page = res.page;
			limit = res.limit;
		} catch (err: unknown) {
			errorMessage = err instanceof Error ? err.message : 'Fehler beim Laden der Dateiliste.';
		} finally {
			isLoading = false;
		}
	}

	onMount(() => {
		const searchParam = pageState.url.searchParams.get('search');
		if (searchParam) {
			filter = searchParam;
		}
		loadFiles();
	});

	let unsubscribeUpload = uploadCompletedTrigger.subscribe((count) => {
		if (count > 0) {
			loadFiles();
		}
	});

	onDestroy(() => {
		unsubscribeUpload();
		if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
	});

	function handleSearchInput(e: Event) {
		const val = (e.target as HTMLInputElement).value;
		filter = val;
		if (searchDebounceTimer) clearTimeout(searchDebounceTimer);
		searchDebounceTimer = setTimeout(() => {
			page = 1;
			loadFiles();
		}, 250);
	}

	function handleSortChange(newSort: string) {
		if (sort === newSort) {
			order = order === 'asc' ? 'desc' : 'asc';
		} else {
			sort = newSort;
			order = 'asc';
		}
		page = 1;
		loadFiles();
	}

	function handlePageChange(newPage: number) {
		page = newPage;
		loadFiles();
	}

	function setCategory(category: 'all' | 'images' | 'docs' | 'videos' | 'audio') {
		activeCategory = category;
		page = 1;
		loadFiles();
	}

	async function handleDownload(f: FileItem) {
		try {
			const res = await downloadFile(f.id);
			if (res.download_url) {
				const a = document.createElement('a');
				a.href = res.download_url;
				a.download = res.filename || f.filename;
				document.body.appendChild(a);
				a.click();
				document.body.removeChild(a);
				addToast(`Download "${f.filename}" gestartet`, 'success');
			}
		} catch (err: unknown) {
			errorMessage = err instanceof Error ? err.message : 'Download fehlgeschlagen.';
			addToast('Download fehlgeschlagen', 'error');
		}
	}

	function promptDelete(f: FileItem) {
		fileToDelete = f;
		bulkDeleteIds = [];
		showDeleteDialog = true;
	}

	function promptBulkDelete(ids: string[]) {
		bulkDeleteIds = ids;
		fileToDelete = null;
		showDeleteDialog = true;
	}

	async function confirmDelete() {
		try {
			if (fileToDelete) {
				await deleteFile(fileToDelete.id);
				addToast(`"${fileToDelete.filename}" gelöscht`, 'success');
			} else if (bulkDeleteIds.length > 0) {
				for (const id of bulkDeleteIds) {
					await deleteFile(id);
				}
				addToast(`${bulkDeleteIds.length} Dateien gelöscht`, 'success');
			}
			showDeleteDialog = false;
			fileToDelete = null;
			bulkDeleteIds = [];
			await loadFiles();
		} catch (err: unknown) {
			errorMessage = err instanceof Error ? err.message : 'Löschen fehlgeschlagen.';
			addToast('Fehler beim Löschen', 'error');
		}
	}

	async function handleRename(f: FileItem, newName: string) {
		try {
			const res = await renameFile(f.id, newName);
			files = files.map((item) => (item.id === f.id ? { ...item, filename: res.filename } : item));
			addToast('Datei erfolgreich umbenannt', 'success');
		} catch (err: unknown) {
			errorMessage = err instanceof Error ? err.message : 'Umbenennen fehlgeschlagen.';
			addToast('Umbenennen fehlgeschlagen', 'error');
			throw err;
		}
	}

	function promptShare(f: FileItem) {
		fileToShare = f;
		showShareDialog = true;
	}

	const categories = [
		{ id: 'all', label: 'Alle Dateien', icon: IconFolder },
		{ id: 'docs', label: 'Dokumente', icon: IconFileText },
		{ id: 'images', label: 'Bilder & Fotos', icon: IconPhoto },
		{ id: 'videos', label: 'Videos', icon: IconVideo },
		{ id: 'audio', label: 'Audiodateien', icon: IconMusic }
	] as const;
</script>

<div class="space-y-6">
	<!-- Seiten-Header mit Upload-Action -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b border-border-light dark:border-border-dark">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-text-light dark:text-text-dark">
				Dateien
			</h1>
			<p class="text-xs sm:text-sm text-muted-light dark:text-muted-dark mt-1">
				Ende-zu-Ende verschlüsselter Dateispeicher (AES-256-GCM) mit Row-Level-Security.
			</p>
		</div>

		<div class="flex items-center gap-2.5">
			<Button
				variant="primary"
				size="md"
				onclick={() => { showUploadModal = !showUploadModal; }}
			>
				<IconUpload size={16} stroke={1.75} class="mr-1.5" />
				<span>Dateien hochladen</span>
			</Button>
		</div>
	</div>

	<!-- Fehlermeldung -->
	{#if errorMessage}
		<div class="p-3.5 rounded-xl bg-danger/10 border border-danger/20 text-danger text-xs font-medium flex items-start gap-2.5" role="alert">
			<IconAlertCircle size={16} stroke={1.75} class="shrink-0 mt-0.5" />
			<span>{errorMessage}</span>
		</div>
	{/if}

	<!-- Aufklappbare Upload-Zone -->
	{#if showUploadModal}
		<div class="p-5 rounded-xl bg-surface-light dark:bg-surface-dark border border-border-light dark:border-border-dark shadow-soft space-y-4 animate-in fade-in duration-150">
			<div class="flex items-center justify-between">
				<h2 class="text-sm font-semibold text-text-light dark:text-text-dark">Direkt-Upload (Tus Resumable)</h2>
				<button
					type="button"
					onclick={() => { showUploadModal = false; }}
					class="p-1 rounded-lg text-muted-light hover:text-text-light dark:text-muted-dark dark:hover:text-text-dark cursor-pointer"
					aria-label="Upload schließen"
				>
					<IconX size={16} stroke={1.75} />
				</button>
			</div>
			<Uploader oncomplete={() => loadFiles()} />
		</div>
	{/if}

	<!-- Zweispaltiges Layout (Seafile-Struktur): Links 200px Ordnerbaum, Rechts Tabelle -->
	<div class="flex flex-col md:flex-row items-start gap-6">
		<!-- Linke Spalte (200px): Ordnerbaum / Kategorien -->
		<div class="w-full md:w-[200px] shrink-0 space-y-1">
			<div class="text-2xs font-semibold uppercase tracking-wider text-muted-light dark:text-muted-dark px-2.5 mb-2">
				Ordner & Filter
			</div>
			<nav class="space-y-0.5">
				{#each categories as cat}
					{@const Icon = cat.icon}
					{@const isActive = activeCategory === cat.id}
					<button
						type="button"
						onclick={() => setCategory(cat.id)}
						class="w-full flex items-center gap-2.5 px-3 py-2 rounded-lg text-xs font-medium transition-colors text-left cursor-pointer {
							isActive
								? 'bg-primary/15 text-primary dark:text-primary-light font-semibold border-l-[3px] border-primary pl-[9px]'
								: 'border-l-[3px] border-transparent text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark hover:bg-slate-100 dark:hover:bg-slate-800/60 pl-[9px]'
						}"
					>
						<Icon class="w-4 h-4 shrink-0" />
						<span>{cat.label}</span>
					</button>
				{/each}
			</nav>
		</div>

		<!-- Rechte Spalte: Toolbar & Tabelle -->
		<div class="flex-1 min-w-0 w-full space-y-4">
			<!-- Toolbar oben: Suche, Grid/List-Toggle, Aktualisieren -->
			<div class="flex items-center justify-between gap-3">
				<div class="relative flex-1 max-w-sm">
					<IconSearch size={16} stroke={1.75} class="absolute left-3 top-1/2 -translate-y-1/2 text-muted-light dark:text-muted-dark pointer-events-none" />
					<input
						type="text"
						placeholder="Filter nach Dateiname..."
						value={filter}
						oninput={handleSearchInput}
						class="w-full pl-9 pr-3 py-2 text-xs rounded-lg border border-border-light dark:border-border-dark bg-surface-light dark:bg-surface-dark text-text-light dark:text-text-dark placeholder-muted-light dark:placeholder-muted-dark focus:outline-hidden focus:border-primary focus:ring-1 focus:ring-primary transition-all"
					/>
				</div>

				<div class="flex items-center gap-2">
					<!-- Grid / List Toggle -->
					<div class="flex items-center rounded-lg border border-border-light dark:border-border-dark bg-surface-light dark:bg-surface-dark p-0.5">
						<button
							type="button"
							onclick={() => (viewMode = 'list')}
							class="p-1.5 rounded-md transition-colors cursor-pointer {viewMode === 'list' ? 'bg-primary/15 text-primary dark:text-primary-light' : 'text-muted-light dark:text-muted-dark hover:text-text-light'}"
							title="Listenansicht"
							aria-label="Listenansicht"
						>
							<IconList size={16} stroke={1.75} />
						</button>
						<button
							type="button"
							onclick={() => (viewMode = 'grid')}
							class="p-1.5 rounded-md transition-colors cursor-pointer {viewMode === 'grid' ? 'bg-primary/15 text-primary dark:text-primary-light' : 'text-muted-light dark:text-muted-dark hover:text-text-light'}"
							title="Kachelansicht"
							aria-label="Kachelansicht"
						>
							<IconLayoutGrid size={16} stroke={1.75} />
						</button>
					</div>

					<!-- Aktualisieren Button -->
					<Button
						variant="secondary"
						size="sm"
						onclick={loadFiles}
						title="Aktualisieren"
						aria-label="Aktualisieren"
					>
						<IconRefresh size={16} stroke={1.75} class={isLoading ? 'animate-spin' : ''} />
					</Button>
				</div>
			</div>

			<!-- Dateitabelle mit Sortierung und Kontextmenue -->
			<FileList
				{files}
				{total}
				{page}
				{limit}
				{sort}
				{order}
				{viewMode}
				{isLoading}
				onsortchange={handleSortChange}
				onpagechange={handlePageChange}
				ondownload={handleDownload}
				onrename={handleRename}
				ondelete={promptDelete}
				onbulkdelete={promptBulkDelete}
				onshare={promptShare}
			/>
		</div>
	</div>
</div>

<!-- Loesch-Bestaetigungsdialog -->
<ConfirmDialog
	open={showDeleteDialog}
	title={bulkDeleteIds.length > 0 ? "Dateien endgültig löschen?" : "Datei endgültig löschen?"}
	message={
		fileToDelete
			? `Möchten Sie "${fileToDelete.filename}" wirklich physisch und endgültig aus 4labscloud löschen?`
			: `Möchten Sie die ${bulkDeleteIds.length} ausgewählten Dateien wirklich endgültig löschen?`
	}
	confirmText="Endgültig löschen"
	danger={true}
	onconfirm={confirmDelete}
	oncancel={() => { showDeleteDialog = false; fileToDelete = null; bulkDeleteIds = []; }}
/>

<!-- Freigabe-Dialog -->
<ShareDialog
	file={fileToShare}
	open={showShareDialog}
	onclose={() => { showShareDialog = false; fileToShare = null; }}
/>

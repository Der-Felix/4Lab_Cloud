<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { formatBytes, formatDate, downloadFile } from '$lib/files';
	import { getThumbnailBlob, assignTag, removeTag, listTags, type PhotoItem, type TagItem } from '$lib/photos';

	export let photos: PhotoItem[] = [];
	export let currentIndex = 0;
	export let onClose: () => void = () => {};

	let showInfo = false;
	let isDownloading = false;
	let newTagName = '';
	let allTags: TagItem[] = [];
	let assignedTags: TagItem[] = [];
	let isLoadingTags = false;
	let imageBlobUrl = '';

	$: currentPhoto = photos[currentIndex];
	$: isVideo = currentPhoto?.mime_type?.startsWith('video/');
	$: hasPrev = currentIndex > 0;
	$: hasNext = currentIndex < photos.length - 1;

	$: if (currentPhoto && !isVideo) {
		getThumbnailBlob(currentPhoto.id).then((url) => {
			imageBlobUrl = url;
		}).catch(() => {
			imageBlobUrl = '';
		});
	}

	onMount(async () => {
		window.addEventListener('keydown', handleKeyDown);
		await loadTags();
	});

	onDestroy(() => {
		if (typeof window !== 'undefined') {
			window.removeEventListener('keydown', handleKeyDown);
		}
	});

	async function loadTags() {
		try {
			isLoadingTags = true;
			allTags = await listTags();
		} catch {
			// Tags optional
		} finally {
			isLoadingTags = false;
		}
	}

	function handleKeyDown(event: KeyboardEvent) {
		if (event.key === 'Escape') {
			onClose();
		} else if (event.key === 'ArrowLeft' && hasPrev) {
			prev();
		} else if (event.key === 'ArrowRight' && hasNext) {
			next();
		}
	}

	function prev() {
		if (hasPrev) {
			currentIndex -= 1;
		}
	}

	function next() {
		if (hasNext) {
			currentIndex += 1;
		}
	}

	async function handleDownload() {
		if (!currentPhoto || isDownloading) return;
		try {
			isDownloading = true;
			const res = await downloadFile(currentPhoto.id);
			const a = document.createElement('a');
			a.href = res.download_url;
			a.download = res.filename;
			document.body.appendChild(a);
			a.click();
			document.body.removeChild(a);
		} catch (err) {
			console.error('Download-Fehler:', err);
		} finally {
			isDownloading = false;
		}
	}

	async function handleAddTag() {
		if (!newTagName.trim() || !currentPhoto) return;
		try {
			await assignTag(currentPhoto.id, { name: newTagName.trim() });
			newTagName = '';
			await loadTags();
		} catch (err) {
			console.error('Tag-Zuweisung fehlgeschlagen:', err);
		}
	}
</script>

<!-- Vollbild Modal-Overlay -->
<div class="fixed inset-0 z-50 flex bg-black/95 backdrop-blur-sm text-white select-none">
	<!-- Hauptbereich Bildansicht -->
	<div class="relative flex-1 flex flex-col h-full overflow-hidden">
		<!-- Obere Toolbar -->
		<div class="flex items-center justify-between px-6 py-4 z-10 bg-linear-to-b from-black/80 to-transparent">
			<div class="flex items-center space-x-3 truncate">
				<span class="text-sm font-medium text-neutral-300">
					{currentIndex + 1} / {photos.length}
				</span>
				<span class="text-neutral-500">|</span>
				<span class="truncate text-sm font-medium text-neutral-200">
					{currentPhoto?.filename}
				</span>
			</div>

			<div class="flex items-center space-x-2">
				<!-- Download Button -->
				<button
					type="button"
					on:click={handleDownload}
					disabled={isDownloading}
					class="p-2 rounded-lg text-neutral-300 hover:text-white hover:bg-white/10 transition-colors disabled:opacity-50"
					title="Herunterladen"
				>
					<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
					</svg>
				</button>

				<!-- Info Toggle -->
				<button
					type="button"
					on:click={() => (showInfo = !showInfo)}
					class="p-2 rounded-lg text-neutral-300 hover:text-white hover:bg-white/10 transition-colors {showInfo ? 'bg-white/15 text-white' : ''}"
					title="Informationen anzeigen"
				>
					<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
					</svg>
				</button>

				<!-- Schliessen Button -->
				<button
					type="button"
					on:click={onClose}
					class="p-2 rounded-lg text-neutral-300 hover:text-white hover:bg-white/10 transition-colors ml-2"
					title="Schliessen (Esc)"
				>
					<svg class="w-5 h-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
					</svg>
				</button>
			</div>
		</div>

		<!-- Bild- / Video-Container -->
		<div class="relative flex-1 flex items-center justify-center p-4">
			{#if currentPhoto}
				{#if isVideo}
					<div class="flex flex-col items-center justify-center p-8 text-center">
						<div class="w-20 h-20 rounded-full bg-white/10 flex items-center justify-center mb-4">
							<svg class="w-10 h-10 fill-white ml-1" viewBox="0 0 20 20">
								<path d="M6.3 2.841A1.5 1.5 0 004 4.11V15.89a1.5 1.5 0 002.3 1.269l9.344-5.89a1.5 1.5 0 000-2.538L6.3 2.84z" />
							</svg>
						</div>
						<p class="text-base font-medium">{currentPhoto.filename}</p>
						<p class="text-sm text-neutral-400 mt-1">{formatBytes(currentPhoto.size_bytes)}</p>
						<button
							type="button"
							on:click={handleDownload}
							class="mt-4 px-4 py-2 rounded-lg bg-primary hover:bg-primary-hover text-sm font-medium transition-colors"
						>
							Video herunterladen
						</button>
					</div>
				{:else}
					<img
						src={imageBlobUrl}
						alt={currentPhoto.filename}
						class="max-h-full max-w-full object-contain rounded-lg shadow-2xl transition-all duration-200"
					/>
				{/if}
			{/if}

			<!-- Vorheriges Bild Button -->
			{#if hasPrev}
				<button
					type="button"
					on:click={prev}
					class="absolute left-4 top-1/2 -translate-y-1/2 p-3 rounded-full bg-black/40 hover:bg-black/80 text-white/80 hover:text-white transition-all backdrop-blur-xs"
					title="Vorheriges Bild (Pfeiltaste links)"
				>
					<svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7" />
					</svg>
				</button>
			{/if}

			<!-- Naechstes Bild Button -->
			{#if hasNext}
				<button
					type="button"
					on:click={next}
					class="absolute right-4 top-1/2 -translate-y-1/2 p-3 rounded-full bg-black/40 hover:bg-black/80 text-white/80 hover:text-white transition-all backdrop-blur-xs"
					title="Naechstes Bild (Pfeiltaste rechts)"
				>
					<svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
						<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
					</svg>
				</button>
			{/if}
		</div>
	</div>

	<!-- Rechte Seitenleiste: Metadaten & Tags -->
	{#if showInfo && currentPhoto}
		<aside class="w-80 h-full bg-neutral-900 border-l border-neutral-800 p-6 flex flex-col space-y-6 overflow-y-auto">
			<div class="flex items-center justify-between border-b border-neutral-800 pb-4">
				<h3 class="text-base font-semibold text-white">Details</h3>
				<button
					type="button"
					on:click={() => (showInfo = false)}
					class="text-neutral-400 hover:text-white text-sm"
				>
					Schliessen
				</button>
			</div>

			<!-- Datei-Metadaten (DSGVO-konform, keine sensiblen EXIF-GPS-Daten) -->
			<div class="space-y-3 text-sm">
				<div>
					<span class="block text-xs text-neutral-400">Dateiname</span>
					<span class="font-medium text-neutral-200 break-all">{currentPhoto.filename}</span>
				</div>

				{#if currentPhoto.width && currentPhoto.height}
					<div>
						<span class="block text-xs text-neutral-400">Aufloesung</span>
						<span class="text-neutral-200">{currentPhoto.width} × {currentPhoto.height} Pixel</span>
					</div>
				{/if}

				<div>
					<span class="block text-xs text-neutral-400">Dateigroesse</span>
					<span class="text-neutral-200">{formatBytes(currentPhoto.size_bytes)}</span>
				</div>

				<div>
					<span class="block text-xs text-neutral-400">Format</span>
					<span class="text-neutral-200">{currentPhoto.mime_type || 'Unbekannt'}</span>
				</div>

				<div>
					<span class="block text-xs text-neutral-400">Datum</span>
					<span class="text-neutral-200">
						{formatDate(currentPhoto.taken_at || currentPhoto.created_at)}
					</span>
				</div>
			</div>

			<!-- Alben & Tags -->
			<div class="space-y-3 border-t border-neutral-800 pt-5">
				<h4 class="text-xs font-semibold uppercase tracking-wider text-neutral-400">Alben & Tags</h4>

				<div class="flex flex-wrap gap-1.5">
					{#each allTags as tag}
						<span
							class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-medium bg-neutral-800 text-neutral-300 border border-neutral-700"
							style={tag.color ? `border-left-color: ${tag.color}; border-left-width: 3px;` : ''}
						>
							{tag.name}
						</span>
					{/each}
				</div>

				<!-- Tag hinzufuegen -->
				<form on:submit|preventDefault={handleAddTag} class="flex items-center gap-2 mt-2">
					<input
						type="text"
						bind:value={newTagName}
						placeholder="Tag / Album hinzufuegen..."
						class="flex-1 rounded-lg bg-neutral-800 border border-neutral-700 px-3 py-1.5 text-xs text-white placeholder-neutral-500 focus:outline-none focus:border-blue-500"
					/>
					<button
						type="submit"
						class="rounded-lg bg-blue-600 px-3 py-1.5 text-xs font-medium text-white hover:bg-blue-700 transition-colors"
					>
						+
					</button>
				</form>
			</div>
		</aside>
	{/if}
</div>

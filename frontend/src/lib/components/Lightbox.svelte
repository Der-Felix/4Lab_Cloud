<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { formatBytes, formatDate, downloadFile } from '$lib/files';
	import {
		getThumbnailBlob,
		assignTag,
		listTags,
		formatAperture,
		formatExposure,
		formatFocalLength,
		formatISO,
		formatCoordinates,
		type PhotoItem,
		type TagItem
	} from '$lib/photos';
	import { getFileExif, type FileExifData } from '$lib/api';
	import {
		IconCamera,
		IconAperture,
		IconClock,
		IconMapPin,
		IconCurrentLocation,
		IconMap,
		IconFocus2,
		IconDownload,
		IconInfoCircle,
		IconX,
		IconChevronLeft,
		IconChevronRight
	} from '@tabler/icons-svelte';

	export let photos: PhotoItem[] = [];
	export let currentIndex = 0;
	export let onClose: () => void = () => {};

	// Rechte Seitenleiste (280px), standardmaessig offen
	let showInfo = true;
	let isDownloading = false;
	let newTagName = '';
	let allTags: TagItem[] = [];
	let isLoadingTags = false;
	let imageBlobUrl = '';

	// EXIF-Zustand
	let exifData: FileExifData | null = null;
	let isLoadingExif = false;
	let exifLoadedForId = '';
	// Zaehler zur Erkennung veralteter EXIF-Antworten (Race Condition beim schnellen Wechseln der Fotos)
	let exifRequestToken = 0;

	$: currentPhoto = photos[currentIndex];
	$: isVideo = currentPhoto?.mime_type?.startsWith('video/');
	$: hasPrev = currentIndex > 0;
	$: hasNext = currentIndex < photos.length - 1;

	$: if (currentPhoto && !isVideo) {
		// Foto-ID vor dem Fetch merken: veraltete Antworten (falls inzwischen weitergeblaettert wurde) werden verworfen
		const requestedId = currentPhoto.id;
		getThumbnailBlob(requestedId)
			.then((url) => {
				if (currentPhoto?.id !== requestedId) return;
				imageBlobUrl = url;
			})
			.catch(() => {
				if (currentPhoto?.id !== requestedId) return;
				imageBlobUrl = '';
			});
	}

	$: if (currentPhoto && currentPhoto.id !== exifLoadedForId) {
		exifLoadedForId = currentPhoto.id;
		loadExif(currentPhoto.id);
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

	async function loadExif(fileId: string) {
		// Token vor dem Await erhoehen und lokal merken, damit eine veraltet eintreffende Antwort
		// (ueberholt durch einen neueren Foto-Wechsel) weder Daten noch den Ladezustand ueberschreibt
		const requestToken = ++exifRequestToken;
		isLoadingExif = true;
		exifData = null;
		try {
			const data = await getFileExif(fileId);
			if (requestToken !== exifRequestToken) return;
			exifData = data;
		} catch {
			if (requestToken !== exifRequestToken) return;
			exifData = null;
		} finally {
			if (requestToken === exifRequestToken) {
				isLoadingExif = false;
			}
		}
	}

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

	// Berechnete EXIF-Werte
	$: rawExif = (exifData?.exif_json || {}) as Record<string, any>;
	$: cameraMake = (rawExif.make as string) || '';
	$: cameraModel = (rawExif.model as string) || '';
	$: cameraName = [cameraMake, cameraModel].filter(Boolean).join(' ');
	$: lensModel = (rawExif.lens_model as string) || '';
	$: fNumberFormatted = formatAperture(rawExif.f_number as string);
	$: exposureFormatted = formatExposure(rawExif.exposure_time as string);
	$: isoFormatted = formatISO(rawExif.iso as number);
	$: focalFormatted = formatFocalLength(rawExif.focal_length as string);

	// GPS Koordinaten (aus Exif-Response oder exif_json)
	$: gpsLat = exifData?.gps_lat ?? (rawExif.gps?.lat as number | undefined);
	$: gpsLon = exifData?.gps_lon ?? (rawExif.gps?.lon as number | undefined);
	$: hasGps = typeof gpsLat === 'number' && typeof gpsLon === 'number';
	$: locationName = exifData?.location_name || null;

	// Prüfen, ob ueberhaupt EXIF / Metadaten existieren
	$: hasAnyExif =
		Boolean(locationName) ||
		hasGps ||
		Boolean(cameraName) ||
		Boolean(lensModel) ||
		Boolean(fNumberFormatted) ||
		Boolean(exposureFormatted) ||
		Boolean(isoFormatted) ||
		Boolean(focalFormatted);
</script>

<!-- Vollbild Modal-Overlay -->
<div class="fixed inset-0 z-50 flex bg-black/95 backdrop-blur-sm text-white select-none overflow-hidden">
	<!-- Hauptbereich Bildansicht -->
	<div class="relative flex-1 flex flex-col h-full overflow-hidden">
		<!-- Obere Toolbar -->
		<div class="flex items-center justify-between px-6 py-4 z-10 bg-linear-to-b from-black/80 to-transparent">
			<div class="flex items-center space-x-3 truncate">
				<span class="text-sm font-medium text-neutral-300 tabular-nums">
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
					class="p-2 rounded-lg text-neutral-300 hover:text-white hover:bg-white/10 transition-colors disabled:opacity-50 cursor-pointer"
					title="Herunterladen"
				>
					<IconDownload size={20} stroke={1.75} />
				</button>

				<!-- Info Toggle Desktop & Mobile -->
				<button
					type="button"
					on:click={() => (showInfo = !showInfo)}
					class="p-2 rounded-lg text-neutral-300 hover:text-white hover:bg-white/10 transition-colors cursor-pointer {showInfo ? 'bg-white/15 text-accent' : ''}"
					title="Informationen anzeigen"
				>
					<IconInfoCircle size={20} stroke={1.75} />
				</button>

				<!-- Schliessen Button -->
				<button
					type="button"
					on:click={onClose}
					class="p-2 rounded-lg text-neutral-300 hover:text-white hover:bg-white/10 transition-colors ml-2 cursor-pointer"
					title="Schliessen (Esc)"
				>
					<IconX size={20} stroke={1.75} />
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
						<p class="text-sm text-neutral-400 mt-1 tabular-nums">{formatBytes(currentPhoto.size_bytes)}</p>
						<button
							type="button"
							on:click={handleDownload}
							class="mt-4 px-4 py-2 rounded-lg bg-primary hover:bg-primary-hover text-sm font-medium transition-colors cursor-pointer"
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
					class="absolute left-4 top-1/2 -translate-y-1/2 p-3 rounded-full bg-black/40 hover:bg-black/80 text-white/80 hover:text-white transition-all backdrop-blur-xs cursor-pointer"
					title="Vorheriges Bild (Pfeiltaste links)"
				>
					<IconChevronLeft size={24} stroke={1.75} />
				</button>
			{/if}

			<!-- Naechstes Bild Button -->
			{#if hasNext}
				<button
					type="button"
					on:click={next}
					class="absolute right-4 top-1/2 -translate-y-1/2 p-3 rounded-full bg-black/40 hover:bg-black/80 text-white/80 hover:text-white transition-all backdrop-blur-xs cursor-pointer"
					title="Naechstes Bild (Pfeiltaste rechts)"
				>
					<IconChevronRight size={24} stroke={1.75} />
				</button>
			{/if}
		</div>

		<!-- Mobile Bottom Toggle Leiste (< 768px) -->
		<div class="md:hidden flex items-center justify-center p-3 z-20 bg-linear-to-t from-black/80 to-transparent">
			<button
				type="button"
				on:click={() => (showInfo = !showInfo)}
				class="px-4 py-1.5 rounded-full bg-surface-dark/90 border border-border-dark text-xs font-medium text-text-dark flex items-center gap-2 shadow-lg cursor-pointer"
			>
				<IconInfoCircle size={15} class={showInfo ? 'text-accent' : ''} />
				<span>{showInfo ? 'Metadaten schließen' : 'Metadaten anzeigen'}</span>
			</button>
		</div>
	</div>

	<!-- 1. Desktop: Rechte Seitenleiste (280px, aufklappbar, standardmaessig offen) -->
	{#if showInfo && currentPhoto}
		<aside class="hidden md:flex w-[280px] h-full bg-surface-dark border-l border-border-dark flex-col overflow-y-auto shrink-0 z-20">
			<!-- Header -->
			<div class="flex items-center justify-between p-4 border-b border-border-dark">
				<div class="flex items-center gap-2">
					<div class="w-0.5 h-4 rounded-full bg-primary shrink-0"></div>
					<h3 class="text-sm font-semibold text-text-dark">Informationen</h3>
				</div>
				<button
					type="button"
					on:click={() => (showInfo = false)}
					class="p-1 rounded-md text-muted-dark hover:text-text-dark hover:bg-white/5 transition-colors cursor-pointer"
					title="Seitenleiste einklappen"
				>
					<IconX size={16} />
				</button>
			</div>

			<div class="p-4 space-y-5 text-xs overflow-y-auto flex-1">
				<!-- EXIF & Aufnahme-Bereich -->
				<div class="space-y-3">
					<h4 class="text-[11px] font-semibold uppercase tracking-wider text-muted-dark">Aufnahme & Gerät</h4>

					{#if isLoadingExif}
						<div class="space-y-2">
							<div class="h-8 bg-slate-800/60 rounded-md animate-skeleton"></div>
							<div class="h-8 bg-slate-800/60 rounded-md animate-skeleton"></div>
						</div>
					{:else if !hasAnyExif}
						<p class="text-muted-dark italic py-1">Keine Metadaten verfügbar</p>
					{:else}
						<!-- Ort / Geocoding -->
						{#if locationName}
							<div class="flex items-start gap-2.5 p-2 rounded-lg bg-bg-dark/50 border border-border-dark">
								<IconMapPin size={16} class="text-accent shrink-0 mt-0.5" />
								<div class="min-w-0 flex-1">
									<span class="block text-[10px] text-muted-dark">Ort</span>
									<span class="font-medium text-text-dark break-words">{locationName}</span>
								</div>
							</div>
						{:else if hasGps}
							<div class="flex items-start gap-2.5 p-2 rounded-lg bg-bg-dark/50 border border-border-dark">
								<IconMapPin size={16} class="text-amber shrink-0 mt-0.5" />
								<div class="min-w-0 flex-1">
									<span class="block text-[10px] text-muted-dark">Ort</span>
									<span class="font-medium text-amber">Ort nicht ermittelt (Geocoding inaktiv)</span>
								</div>
							</div>
						{/if}

						<!-- Kamera -->
						{#if cameraName}
							<div class="flex items-start gap-2.5">
								<IconCamera size={16} class="text-primary-light shrink-0 mt-0.5" />
								<div class="min-w-0 flex-1">
									<span class="block text-[10px] text-muted-dark">Kamera</span>
									<span class="font-medium text-text-dark">{cameraName}</span>
								</div>
							</div>
						{/if}

						<!-- Objektiv -->
						{#if lensModel}
							<div class="flex items-start gap-2.5">
								<IconFocus2 size={16} class="text-primary-light shrink-0 mt-0.5" />
								<div class="min-w-0 flex-1">
									<span class="block text-[10px] text-muted-dark">Objektiv</span>
									<span class="font-medium text-text-dark">{lensModel}</span>
								</div>
							</div>
						{/if}

						<!-- Aufnahmewerte (Blende, Verschlusszeit, ISO, Brennweite) -->
						{#if fNumberFormatted || exposureFormatted || isoFormatted || focalFormatted}
							<div class="grid grid-cols-2 gap-2 pt-2 border-t border-border-dark">
								{#if fNumberFormatted}
									<div class="flex items-center gap-1.5 text-text-dark">
										<IconAperture size={14} class="text-accent shrink-0" />
										<span class="tabular-nums">{fNumberFormatted}</span>
									</div>
								{/if}
								{#if exposureFormatted}
									<div class="flex items-center gap-1.5 text-text-dark">
										<IconClock size={14} class="text-accent shrink-0" />
										<span class="tabular-nums">{exposureFormatted}</span>
									</div>
								{/if}
								{#if isoFormatted}
									<div class="flex items-center gap-1.5 text-text-dark">
										<span class="text-[9px] font-bold text-muted-dark px-1 bg-border-dark rounded">ISO</span>
										<span class="tabular-nums">{isoFormatted.replace('ISO ', '')}</span>
									</div>
								{/if}
								{#if focalFormatted}
									<div class="flex items-center gap-1.5 text-text-dark">
										<span class="text-[9px] font-bold text-muted-dark px-1 bg-border-dark rounded">FL</span>
										<span class="tabular-nums">{focalFormatted}</span>
									</div>
								{/if}
							</div>
						{/if}

						<!-- GPS-Koordinaten & Button "Auf Karte zeigen" -->
						{#if hasGps}
							<div class="pt-2 border-t border-border-dark space-y-2">
								<div class="flex items-start gap-2 text-text-dark">
									<IconCurrentLocation size={15} class="text-accent shrink-0 mt-0.5" />
									<span class="tabular-nums text-[11px] text-muted-dark">
										{formatCoordinates(gpsLat, gpsLon)}
									</span>
								</div>

								<a
									href="/photos/map?photo={currentPhoto.id}"
									class="flex items-center justify-center gap-1.5 w-full py-2 px-3 rounded-lg bg-accent/15 text-accent hover:bg-accent/25 border border-accent/30 text-xs font-medium transition-colors"
								>
									<IconMap size={15} />
									<span>Auf Karte zeigen</span>
								</a>
							</div>
						{/if}
					{/if}
				</div>

				<!-- Datei-Details -->
				<div class="space-y-2.5 border-t border-border-dark pt-4">
					<h4 class="text-[11px] font-semibold uppercase tracking-wider text-muted-dark">Datei</h4>

					<div>
						<span class="block text-[10px] text-muted-dark">Dateiname</span>
						<span class="font-medium text-text-dark break-all">{currentPhoto.filename}</span>
					</div>

					{#if currentPhoto.width && currentPhoto.height}
						<div>
							<span class="block text-[10px] text-muted-dark">Auflösung</span>
							<span class="text-text-dark tabular-nums">{currentPhoto.width} × {currentPhoto.height} px</span>
						</div>
					{/if}

					<div>
						<span class="block text-[10px] text-muted-dark">Dateigröße</span>
						<span class="text-text-dark tabular-nums">{formatBytes(currentPhoto.size_bytes)}</span>
					</div>

					<div>
						<span class="block text-[10px] text-muted-dark">Aufnahme- / Erstelldatum</span>
						<span class="text-text-dark tabular-nums">
							{formatDate(currentPhoto.taken_at || currentPhoto.created_at)}
						</span>
					</div>
				</div>

				<!-- Alben & Tags -->
				<div class="space-y-2.5 border-t border-border-dark pt-4">
					<h4 class="text-[11px] font-semibold uppercase tracking-wider text-muted-dark">Alben & Tags</h4>

					<div class="flex flex-wrap gap-1.5">
						{#each allTags as tag}
							<span
								class="inline-flex items-center px-2 py-0.5 rounded-md text-[11px] font-medium bg-bg-dark text-neutral-300 border border-border-dark"
								style={tag.color ? `border-left-color: ${tag.color}; border-left-width: 3px;` : ''}
							>
								{tag.name}
							</span>
						{/each}
					</div>

					<form on:submit|preventDefault={handleAddTag} class="flex items-center gap-1.5 mt-2">
						<input
							type="text"
							bind:value={newTagName}
							placeholder="Neues Tag..."
							class="flex-1 rounded-lg bg-bg-dark border border-border-dark px-2.5 py-1 text-xs text-text-dark placeholder-muted-dark focus:outline-hidden focus:border-primary"
						/>
						<button
							type="submit"
							class="rounded-lg bg-primary px-2.5 py-1 text-xs font-medium text-white hover:bg-primary-hover transition-colors cursor-pointer"
						>
							+
						</button>
					</form>
				</div>
			</div>
		</aside>
	{/if}

	<!-- 2. Mobile: Bottom-Sheet (Slide-up, < 768px) -->
	{#if showInfo && currentPhoto}
		<div
			class="md:hidden fixed inset-x-0 bottom-0 max-h-[75vh] bg-surface-dark border-t border-border-dark rounded-t-2xl shadow-2xl z-40 flex flex-col overflow-hidden animate-in slide-in-from-bottom duration-300"
		>
			<div class="flex items-center justify-between px-4 py-3 border-b border-border-dark bg-surface-dark/95 sticky top-0">
				<div class="flex items-center gap-2">
					<div class="w-0.5 h-4 rounded-full bg-primary shrink-0"></div>
					<h3 class="text-sm font-semibold text-text-dark">Metadaten</h3>
				</div>
				<button
					type="button"
					on:click={() => (showInfo = false)}
					class="p-1 rounded-md text-muted-dark hover:text-text-dark cursor-pointer"
					aria-label="Schließen"
				>
					<IconX size={18} />
				</button>
			</div>

			<div class="p-4 space-y-4 text-xs overflow-y-auto">
				{#if isLoadingExif}
					<div class="space-y-2">
						<div class="h-8 bg-slate-800/60 rounded-md animate-skeleton"></div>
						<div class="h-8 bg-slate-800/60 rounded-md animate-skeleton"></div>
					</div>
				{:else if !hasAnyExif}
					<p class="text-muted-dark italic">Keine Metadaten verfügbar</p>
				{:else}
					<!-- Ort / Geocoding -->
					{#if locationName}
						<div class="flex items-start gap-2.5 p-2 rounded-lg bg-bg-dark/50 border border-border-dark">
							<IconMapPin size={16} class="text-accent shrink-0 mt-0.5" />
							<div class="min-w-0 flex-1">
								<span class="block text-[10px] text-muted-dark">Ort</span>
								<span class="font-medium text-text-dark">{locationName}</span>
							</div>
						</div>
					{:else if hasGps}
						<div class="flex items-start gap-2.5 p-2 rounded-lg bg-bg-dark/50 border border-border-dark">
							<IconMapPin size={16} class="text-amber shrink-0 mt-0.5" />
							<div class="min-w-0 flex-1">
								<span class="block text-[10px] text-muted-dark">Ort</span>
								<span class="font-medium text-amber">Ort nicht ermittelt (Geocoding inaktiv)</span>
							</div>
						</div>
					{/if}

					<!-- Kamera & Objektiv -->
					{#if cameraName || lensModel}
						<div class="space-y-2">
							{#if cameraName}
								<div class="flex items-center gap-2">
									<IconCamera size={16} class="text-primary-light shrink-0" />
									<span class="text-text-dark font-medium">{cameraName}</span>
								</div>
							{/if}
							{#if lensModel}
								<div class="flex items-center gap-2">
									<IconFocus2 size={16} class="text-primary-light shrink-0" />
									<span class="text-text-dark font-medium">{lensModel}</span>
								</div>
							{/if}
						</div>
					{/if}

					<!-- Aufnahmewerte -->
					{#if fNumberFormatted || exposureFormatted || isoFormatted || focalFormatted}
						<div class="grid grid-cols-2 gap-2 pt-2 border-t border-border-dark">
							{#if fNumberFormatted}
								<div class="flex items-center gap-1.5 text-text-dark">
									<IconAperture size={14} class="text-accent" />
									<span>{fNumberFormatted}</span>
								</div>
							{/if}
							{#if exposureFormatted}
								<div class="flex items-center gap-1.5 text-text-dark">
									<IconClock size={14} class="text-accent" />
									<span>{exposureFormatted}</span>
								</div>
							{/if}
							{#if isoFormatted}
								<div class="flex items-center gap-1.5 text-text-dark">
									<span class="text-[9px] font-bold text-muted-dark px-1 bg-border-dark rounded">ISO</span>
									<span>{isoFormatted.replace('ISO ', '')}</span>
								</div>
							{/if}
							{#if focalFormatted}
								<div class="flex items-center gap-1.5 text-text-dark">
									<span class="text-[9px] font-bold text-muted-dark px-1 bg-border-dark rounded">FL</span>
									<span>{focalFormatted}</span>
								</div>
							{/if}
						</div>
					{/if}

					<!-- GPS & Button "Auf Karte zeigen" -->
					{#if hasGps}
						<div class="pt-2 border-t border-border-dark space-y-2">
							<div class="flex items-center gap-2 text-muted-dark text-[11px]">
								<IconCurrentLocation size={14} class="text-accent" />
								<span>{formatCoordinates(gpsLat, gpsLon)}</span>
							</div>
							<a
								href="/photos/map?photo={currentPhoto.id}"
								class="flex items-center justify-center gap-1.5 w-full py-2 px-3 rounded-lg bg-accent/15 text-accent border border-accent/30 text-xs font-medium"
							>
								<IconMap size={15} />
								<span>Auf Karte zeigen</span>
							</a>
						</div>
					{/if}
				{/if}

				<!-- Datei-Details -->
				<div class="pt-2 border-t border-border-dark space-y-1 text-muted-dark">
					<p><span class="text-text-dark">{currentPhoto.filename}</span> ({formatBytes(currentPhoto.size_bytes)})</p>
					<p>{formatDate(currentPhoto.taken_at || currentPhoto.created_at)}</p>
				</div>
			</div>
		</div>
	{/if}
</div>

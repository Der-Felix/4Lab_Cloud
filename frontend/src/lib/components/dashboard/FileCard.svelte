<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import type { FileItem } from '$lib/files';
	import { formatRelativeTime } from '$lib/files';
	import { getThumbnailBlob, revokeThumbnailBlob } from '$lib/photos';
	import {
		FileText,
		FileSpreadsheet,
		FileCode,
		FileAudio,
		FileVideo,
		Image as ImageIcon,
		File as FileIcon,
		Archive,
		MoreVertical,
		Download,
		Share2,
		Trash2
	} from '@lucide/svelte';

	interface Props {
		file: FileItem;
		ondownload?: (file: FileItem) => void;
		onshare?: (file: FileItem) => void;
		ondelete?: (file: FileItem) => void;
	}

	let { file, ondownload, onshare, ondelete }: Props = $props();

	let blobUrl = $state<string | null>(null);
	let isImage = $derived(file.mime_type?.startsWith('image/'));
	let isLoaded = $state(false);
	let hasError = $state(false);
	let showMenu = $state(false);
	let menuRef = $state<HTMLDivElement | null>(null);

	// Dateityp-Ermittlung mit kontrollierten Kategorie-Farben (Amber, Teal, Rose, Violet, Slate)
	let typeInfo = $derived.by(() => {
		const m = file.mime_type?.toLowerCase() || '';
		if (m.includes('pdf')) {
			return { label: 'PDF', icon: FileText, colorClass: 'text-amber', badgeClass: 'text-amber bg-amber/10' };
		}
		if (m.includes('sheet') || m.includes('excel') || m.includes('csv')) {
			return { label: 'Tabelle', icon: FileSpreadsheet, colorClass: 'text-amber', badgeClass: 'text-amber bg-amber/10' };
		}
		if (m.includes('word') || m.includes('document') || m.includes('text/plain')) {
			return { label: 'Dokument', icon: FileText, colorClass: 'text-amber', badgeClass: 'text-amber bg-amber/10' };
		}
		if (m.startsWith('image/')) {
			return { label: 'Bild', icon: ImageIcon, colorClass: 'text-accent', badgeClass: 'text-accent bg-accent/10' };
		}
		if (m.startsWith('video/')) {
			return { label: 'Video', icon: FileVideo, colorClass: 'text-rose', badgeClass: 'text-rose bg-rose/10' };
		}
		if (m.startsWith('audio/')) {
			return { label: 'Audio', icon: FileAudio, colorClass: 'text-violet', badgeClass: 'text-violet bg-violet/10' };
		}
		if (m.includes('zip') || m.includes('tar') || m.includes('gz') || m.includes('rar') || m.includes('7z')) {
			return { label: 'Archiv', icon: Archive, colorClass: 'text-slate', badgeClass: 'text-slate bg-slate/10' };
		}
		if (m.includes('json') || m.includes('javascript') || m.includes('html') || m.includes('code')) {
			return { label: 'Code', icon: FileCode, colorClass: 'text-slate', badgeClass: 'text-slate bg-slate/10' };
		}
		return { label: 'Datei', icon: FileIcon, colorClass: 'text-muted-light dark:text-muted-dark', badgeClass: 'text-muted-light dark:text-muted-dark bg-slate-100 dark:bg-bg-dark' };
	});

	function handleClickOutside(e: MouseEvent) {
		if (menuRef && !menuRef.contains(e.target as Node)) {
			showMenu = false;
		}
	}

	onMount(() => {
		if (isImage) {
			getThumbnailBlob(file.id)
				.then((url) => {
					blobUrl = url;
				})
				.catch(() => {
					hasError = true;
				});
		}

		window.addEventListener('click', handleClickOutside);
		return () => {
			window.removeEventListener('click', handleClickOutside);
		};
	});

	onDestroy(() => {
		if (file?.id) {
			revokeThumbnailBlob(file.id);
		}
	});

	function handleContextMenu(e: MouseEvent) {
		e.preventDefault();
		showMenu = true;
	}
</script>

<div
	class="group relative flex flex-col justify-between rounded-xl border border-border-light dark:border-border-dark bg-surface-light dark:bg-surface-dark card-depth shadow-depth shadow-depth-hover select-none overflow-hidden"
	oncontextmenu={handleContextMenu}
	role="group"
>
	<!-- Thumbnail-Bereich: aspect-square, FULL-BLEED ohne Innenabstand -->
	<div class="relative w-full aspect-square overflow-hidden flex items-center justify-center bg-slate-100 dark:bg-bg-dark border-b border-border-light dark:border-border-dark">
		{#if isImage && blobUrl && !hasError}
			<img
				src={blobUrl}
				alt={file.filename}
				class="w-full h-full object-cover group-hover:scale-105 transition-transform duration-200 {isLoaded ? 'opacity-100' : 'opacity-0'}"
				loading="lazy"
				onload={() => (isLoaded = true)}
				onerror={() => (hasError = true)}
			/>
			{#if !isLoaded}
				<div class="absolute inset-0 bg-slate-200/70 dark:bg-slate-800/60 animate-skeleton flex items-center justify-center">
					<ImageIcon class="w-12 h-12 text-accent/50 opacity-40" />
				</div>
			{/if}
		{:else if isImage && !blobUrl && !hasError}
			<!-- Skeleton waehrend des Ladens des Blobs -->
			<div class="w-full h-full bg-slate-200/70 dark:bg-slate-800/60 animate-skeleton flex items-center justify-center">
				<ImageIcon class="w-12 h-12 text-accent/50 opacity-40" />
			</div>
		{:else}
			<!-- Grosses 48px Icon in Kategorie-Farbe auf neutralem Hintergrund (kein bunter Kachel-Hintergrund) -->
			{@const TypeIcon = typeInfo.icon}
			<div class="flex items-center justify-center w-full h-full">
				<TypeIcon class="w-12 h-12 stroke-[1.5] {typeInfo.colorClass} opacity-90 group-hover:scale-110 transition-all duration-200" />
			</div>
		{/if}

		<!-- 3-Punkte-Menue Button (erscheint bei Hover oder wenn Menue aktiv) -->
		<div class="absolute top-2 right-2 opacity-0 group-hover:opacity-100 focus-within:opacity-100 transition-opacity" bind:this={menuRef}>
			<button
				type="button"
				onclick={(e) => { e.stopPropagation(); showMenu = !showMenu; }}
				class="p-1.5 rounded-lg bg-surface-light/95 dark:bg-surface-dark/95 text-text-light dark:text-text-dark hover:bg-surface-light dark:hover:bg-surface-dark shadow-soft border border-border-light dark:border-border-dark cursor-pointer backdrop-blur-xs"
				title="Optionen"
				aria-label="Optionen"
			>
				<MoreVertical class="w-4 h-4" />
			</button>

			{#if showMenu}
				<div class="absolute right-0 top-full mt-1 w-36 rounded-lg bg-surface-light dark:bg-surface-dark border border-border-light dark:border-border-dark shadow-card py-1 z-20 text-xs">
					{#if ondownload}
						<button
							type="button"
							onclick={() => { showMenu = false; ondownload(file); }}
							class="w-full px-3 py-1.5 text-left flex items-center gap-2 text-text-light dark:text-text-dark hover:bg-slate-100 dark:hover:bg-bg-dark cursor-pointer"
						>
							<Download class="w-3.5 h-3.5 text-muted-light dark:text-muted-dark" />
							<span>Herunterladen</span>
						</button>
					{/if}
					{#if onshare}
						<button
							type="button"
							onclick={() => { showMenu = false; onshare(file); }}
							class="w-full px-3 py-1.5 text-left flex items-center gap-2 text-text-light dark:text-text-dark hover:bg-slate-100 dark:hover:bg-bg-dark cursor-pointer"
						>
							<Share2 class="w-3.5 h-3.5 text-muted-light dark:text-muted-dark" />
							<span>Freigeben</span>
						</button>
					{/if}
					{#if ondelete}
						<button
							type="button"
							onclick={() => { showMenu = false; ondelete(file); }}
							class="w-full px-3 py-1.5 text-left flex items-center gap-2 text-danger hover:bg-danger/10 cursor-pointer"
						>
							<Trash2 class="w-3.5 h-3.5" />
							<span>Löschen</span>
						</button>
					{/if}
				</div>
			{/if}
		</div>
	</div>

	<!-- Unterer Info-Bereich: Dateiname & Metadaten -->
	<div class="p-3 min-w-0">
		<p class="font-bold text-xs text-text-light dark:text-text-dark truncate leading-tight group-hover:text-primary transition-colors" title={file.filename}>
			{file.filename}
		</p>
		<p class="text-[11px] text-muted-light dark:text-muted-dark mt-1.5 flex items-center gap-1.5">
			<span class="px-1.5 py-0.5 rounded text-[10px] font-semibold {typeInfo.badgeClass}">{typeInfo.label}</span>
			<span>·</span>
			<span class="tabular-nums">{formatRelativeTime(file.created_at)}</span>
		</p>
	</div>
</div>

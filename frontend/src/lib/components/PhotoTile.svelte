<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { getThumbnailBlob, revokeThumbnailBlob, type PhotoItem } from '$lib/photos';
	import { formatDate } from '$lib/files';
	import { Image, Play, AlertCircle } from '@lucide/svelte';

	interface Props {
		photo: PhotoItem;
		onSelect?: (photo: PhotoItem) => void;
	}

	let { photo, onSelect }: Props = $props();

	let element = $state<HTMLButtonElement | null>(null);
	let isVisible = $state(false);
	let isLoaded = $state(false);
	let hasError = $state(false);
	let blobUrl = $state<string | null>(null);

	let isVideo = $derived(photo.mime_type?.startsWith('video/'));

	async function loadThumbnail() {
		try {
			const url = await getThumbnailBlob(photo.id);
			blobUrl = url;
		} catch {
			hasError = true;
		}
	}

	onMount(() => {
		if (typeof IntersectionObserver === 'undefined') {
			isVisible = true;
			loadThumbnail();
			return;
		}

		const observer = new IntersectionObserver(
			(entries) => {
				for (const entry of entries) {
					if (entry.isIntersecting) {
						isVisible = true;
						loadThumbnail();
						observer.disconnect();
						break;
					}
				}
			},
			{ rootMargin: '200px' }
		);

		if (element) {
			observer.observe(element);
		}

		return () => {
			observer.disconnect();
		};
	});

	onDestroy(() => {
		if (photo?.id) {
			revokeThumbnailBlob(photo.id);
		}
	});

	function handleClick() {
		onSelect?.(photo);
	}
</script>

<button
	bind:this={element}
	type="button"
	class="group relative aspect-square w-full overflow-hidden rounded-xl bg-surface-light dark:bg-surface-dark border border-border-light dark:border-border-dark focus:ring-2 focus:ring-primary focus:outline-hidden cursor-pointer"
	onclick={handleClick}
	title={photo.filename}
>
	<!-- Skeleton-Loader waehrend des Ladens -->
	{#if (!isLoaded || !blobUrl) && !hasError}
		<div class="absolute inset-0 bg-slate-200/70 dark:bg-slate-800/60 animate-skeleton flex items-center justify-center">
			<Image class="w-6 h-6 text-muted-light dark:text-muted-dark opacity-40" />
		</div>
	{/if}

	<!-- Bild laden sobald Blob-URL bereitsteht -->
	{#if blobUrl && !hasError}
		<img
			src={blobUrl}
			alt={photo.filename}
			class="h-full w-full object-cover transition-transform duration-300 group-hover:scale-105 {isLoaded ? 'opacity-100' : 'opacity-0'}"
			onload={() => (isLoaded = true)}
			onerror={() => (hasError = true)}
		/>
	{/if}

	<!-- Fallback bei Fehler -->
	{#if hasError}
		<div class="flex h-full w-full flex-col items-center justify-center p-3 text-center text-muted-light dark:text-muted-dark">
			<AlertCircle class="w-6 h-6 mb-1 opacity-60 text-warning" />
			<span class="text-[11px] truncate max-w-full">{photo.filename}</span>
		</div>
	{/if}

	<!-- Video-Indikator -->
	{#if isVideo}
		<div class="absolute top-2 right-2 rounded-md bg-black/75 px-1.5 py-0.5 text-[10px] font-medium text-white backdrop-blur-xs flex items-center gap-1">
			<Play class="w-3 h-3 fill-current" />
			<span>Video</span>
		</div>
	{/if}

	<!-- Hover-Overlay: Filename + Datum -->
	<div class="absolute inset-0 bg-gradient-to-t from-black/85 via-black/30 to-transparent opacity-0 transition-opacity duration-150 group-hover:opacity-100 flex flex-col justify-end p-3 text-left">
		<span class="truncate text-xs font-semibold text-white leading-tight">{photo.filename}</span>
		<span class="text-[11px] text-slate-300 tabular-nums mt-0.5">{formatDate(photo.created_at)}</span>
	</div>
</button>

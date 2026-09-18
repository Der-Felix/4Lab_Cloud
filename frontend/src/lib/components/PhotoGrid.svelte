<script lang="ts">
	import { groupPhotosByDate, type PhotoItem } from '$lib/photos';
	import PhotoTile from './PhotoTile.svelte';
	import { Image } from '@lucide/svelte';

	interface Props {
		photos?: PhotoItem[];
		onSelectPhoto?: (photo: PhotoItem) => void;
	}

	let { photos = [], onSelectPhoto }: Props = $props();

	let groups = $derived(groupPhotosByDate(photos));
</script>

{#if photos.length === 0}
	<div class="flex flex-col items-center justify-center py-20 text-center">
		<div class="w-12 h-12 rounded-full bg-slate-100 dark:bg-slate-800 flex items-center justify-center text-muted-light dark:text-muted-dark mb-3">
			<Image class="w-6 h-6 opacity-60" />
		</div>
		<p class="text-sm font-semibold text-text-light dark:text-text-dark">Keine Fotos oder Medien gefunden</p>
		<p class="text-xs text-muted-light dark:text-muted-dark mt-1">Laden Sie Bilder hoch oder passen Sie Ihre Suche/Filter an.</p>
	</div>
{:else}
	<div class="space-y-8">
		{#each groups as group}
			<div class="space-y-3">
				<div class="flex items-center gap-2">
					<h3 class="text-xs font-semibold uppercase tracking-wider text-muted-light dark:text-muted-dark">
						{group.title}
					</h3>
					<span class="text-[11px] text-muted-light dark:text-muted-dark font-medium tabular-nums">({group.photos.length})</span>
				</div>

				<!-- 4-Spalten-Grid -->
				<div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-4 gap-3.5">
					{#each group.photos as photo (photo.id)}
						<PhotoTile {photo} onSelect={onSelectPhoto} />
					{/each}
				</div>
			</div>
		{/each}
	</div>
{/if}

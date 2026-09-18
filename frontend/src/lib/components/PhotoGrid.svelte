<script lang="ts">
	import { groupByTimeline, type PhotoItem } from '$lib/photos';
	import PhotoTile from './PhotoTile.svelte';
	import { IconPhoto } from '@tabler/icons-svelte';

	interface Props {
		photos?: PhotoItem[];
		onSelectPhoto?: (photo: PhotoItem) => void;
	}

	let { photos = [], onSelectPhoto }: Props = $props();

	let timelineMonths = $derived(groupByTimeline(photos));
</script>

{#if photos.length === 0}
	<div class="flex flex-col items-center justify-center py-20 text-center">
		<div class="w-12 h-12 rounded-full bg-slate-100 dark:bg-slate-800 flex items-center justify-center text-muted-light dark:text-muted-dark mb-3">
			<IconPhoto class="w-6 h-6 opacity-60" />
		</div>
		<p class="text-sm font-semibold text-text-light dark:text-text-dark">Keine Fotos oder Medien gefunden</p>
		<p class="text-xs text-muted-light dark:text-muted-dark mt-1">Laden Sie Bilder hoch oder passen Sie Ihre Suche/Filter an.</p>
	</div>
{:else}
	<div class="space-y-10">
		{#each timelineMonths as monthGroup (monthGroup.rawMonth)}
			<div class="space-y-6">
				<!-- Monat-Header: groß, h2, sticky beim Scrollen -->
				<div class="sticky top-0 z-10 -mx-4 px-4 py-2.5 bg-bg-light/95 dark:bg-bg-dark/95 backdrop-blur-md border-b border-border-light dark:border-border-dark flex items-center gap-2.5 shadow-xs">
					<div class="w-0.5 h-4.5 rounded-full bg-primary shrink-0"></div>
					<h2 class="text-base sm:text-lg font-bold tracking-tight text-text-light dark:text-text-dark">
						{monthGroup.month}
					</h2>
				</div>

				<!-- Tag-Untergruppen -->
				<div class="space-y-6">
					{#each monthGroup.days as dayGroup (dayGroup.rawDate)}
						<div class="space-y-3">
							<div class="flex items-center gap-2">
								<h3 class="text-xs font-semibold uppercase tracking-wider text-muted-light dark:text-muted-dark">
									{dayGroup.date}
								</h3>
								<span class="text-[11px] text-muted-light dark:text-muted-dark font-medium tabular-nums">
									({dayGroup.photos.length})
								</span>
							</div>

							<!-- 4-Spalten-Grid (responsive sm:3, md:4, lg:4) -->
							<div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-4 gap-3.5">
								{#each dayGroup.photos as photo (photo.id)}
									<PhotoTile {photo} onSelect={onSelectPhoto} />
								{/each}
							</div>
						</div>
					{/each}
				</div>
			</div>
		{/each}
	</div>
{/if}

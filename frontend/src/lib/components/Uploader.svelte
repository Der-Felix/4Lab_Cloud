<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import '@uppy/core/css/style.min.css';
	import '@uppy/dashboard/css/style.min.css';
	import { createUploader } from '$lib/uploader';

	let { oncomplete }: { oncomplete?: () => void } = $props();

	let containerId = 'uppy-dashboard-' + Math.random().toString(36).substring(2, 9);
	let uppyInstance: ReturnType<typeof createUploader> | null = null;

	onMount(() => {
		uppyInstance = createUploader('#' + containerId);
		if (oncomplete) {
			uppyInstance.on('complete', () => {
				oncomplete();
			});
		}
	});

	onDestroy(() => {
		if (uppyInstance) {
			try {
				uppyInstance.destroy();
			} catch {
				// Ignorieren falls bereits geschlossen
			}
			uppyInstance = null;
		}
	});
</script>

<div class="uppy-wrapper rounded-xl overflow-hidden border border-slate-200 dark:border-slate-800 shadow-sm bg-white dark:bg-slate-900">
	<div id={containerId}></div>
</div>

<style>
	:global(.uppy-Dashboard-inner) {
		border: none !important;
		border-radius: 0.75rem !important;
	}
</style>

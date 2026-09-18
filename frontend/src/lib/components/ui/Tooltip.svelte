<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		text: string;
		position?: 'top' | 'right' | 'bottom' | 'left';
		class?: string;
		children?: Snippet;
	}

	let {
		text,
		position = 'top',
		class: customClass = '',
		children
	}: Props = $props();

	let positionClasses = $derived(
		position === 'right'
			? 'left-full top-1/2 -translate-y-1/2 ml-2'
			: position === 'bottom'
				? 'top-full left-1/2 -translate-x-1/2 mt-2'
				: position === 'left'
					? 'right-full top-1/2 -translate-y-1/2 mr-2'
					: 'bottom-full left-1/2 -translate-x-1/2 mb-2'
	);
</script>

<div class="relative group inline-flex {customClass}">
	{@render children?.()}
	{#if text}
		<div
			class="pointer-events-none absolute z-50 whitespace-nowrap rounded-md bg-bg-dark px-2.5 py-1 text-xs font-medium text-white shadow-lg border border-border-dark opacity-0 transition-opacity duration-150 group-hover:opacity-100 {positionClasses}"
			role="tooltip"
		>
			{text}
		</div>
	{/if}
</div>

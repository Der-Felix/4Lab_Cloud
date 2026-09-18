<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		align?: 'left' | 'right';
		width?: string;
		class?: string;
		trigger?: Snippet;
		children?: Snippet;
	}

	let {
		align = 'right',
		width = 'w-52',
		class: customClass = '',
		trigger,
		children
	}: Props = $props();

	let isOpen = $state(false);
	let containerRef: HTMLDivElement | null = $state(null);

	function toggle() {
		isOpen = !isOpen;
	}

	function close() {
		isOpen = false;
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && isOpen) {
			close();
		}
	}

	function handleWindowClick(event: MouseEvent) {
		if (isOpen && containerRef && !containerRef.contains(event.target as Node)) {
			close();
		}
	}
</script>

<svelte:window onkeydown={handleKeydown} onclick={handleWindowClick} />

<div class="relative inline-block text-left {customClass}" bind:this={containerRef}>
	<!-- Trigger -->
	<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
	<div onclick={toggle}>
		{@render trigger?.()}
	</div>

	<!-- Menu dropdown -->
	{#if isOpen}
		<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
		<div
			class="absolute z-50 mt-1.5 {width} rounded-xl bg-surface-light dark:bg-surface-dark border border-border-light dark:border-border-dark shadow-xl py-1.5 focus:outline-hidden animate-in fade-in zoom-in-95 duration-100 {align === 'right' ? 'right-0' : 'left-0'}"
			role="menu"
			tabindex="-1"
			onclick={close}
		>
			{@render children?.()}
		</div>
	{/if}
</div>

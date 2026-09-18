<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		variant?: 'primary' | 'secondary' | 'ghost' | 'danger';
		size?: 'sm' | 'md' | 'lg';
		type?: 'button' | 'submit' | 'reset';
		disabled?: boolean;
		loading?: boolean;
		href?: string;
		class?: string;
		title?: string;
		'aria-label'?: string;
		onclick?: (event: MouseEvent) => void;
		children?: Snippet;
	}

	let {
		variant = 'primary',
		size = 'md',
		type = 'button',
		disabled = false,
		loading = false,
		href = undefined,
		class: customClass = '',
		title = undefined,
		'aria-label': ariaLabel = undefined,
		onclick,
		children
	}: Props = $props();

	let variantClasses = $derived(
		variant === 'primary'
			? 'bg-primary hover:bg-primary-hover text-white shadow-xs border border-transparent active:scale-[0.98]'
			: variant === 'secondary'
				? 'bg-surface-light dark:bg-surface-dark text-primary border border-border-light dark:border-border-dark hover:bg-slate-100 dark:hover:bg-bg-dark shadow-soft active:scale-[0.98]'
				: variant === 'danger'
					? 'bg-danger hover:bg-danger-hover text-white shadow-xs border border-transparent active:scale-[0.98]'
					: 'bg-transparent text-primary hover:bg-primary/10 active:scale-[0.98]'
	);

	let sizeClasses = $derived(
		size === 'sm'
			? 'px-2.5 py-1.5 text-xs rounded-lg gap-1.5'
			: size === 'lg'
				? 'px-5 py-2.5 text-base rounded-lg gap-2.5'
				: 'px-4 py-2 text-sm rounded-lg gap-2'
	);
</script>

{#if href}
	<a
		{href}
		{title}
		aria-label={ariaLabel}
		class="inline-flex items-center justify-center font-medium transition-all duration-150 select-none cursor-pointer {variantClasses} {sizeClasses} {customClass} {disabled ? 'opacity-50 pointer-events-none' : ''}"
	>
		{@render children?.()}
	</a>
{:else}
	<button
		{type}
		{title}
		aria-label={ariaLabel}
		disabled={disabled || loading}
		{onclick}
		class="inline-flex items-center justify-center font-medium transition-all duration-150 select-none cursor-pointer {variantClasses} {sizeClasses} {customClass} {disabled || loading ? 'opacity-50 pointer-events-none' : ''}"
	>
		{#if loading}
			<svg class="animate-spin -ml-1 mr-2 h-4 w-4 text-current" fill="none" viewBox="0 0 24 24">
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
				<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z"></path>
			</svg>
		{/if}
		{@render children?.()}
	</button>
{/if}

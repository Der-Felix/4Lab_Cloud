<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		variant?: 'primary' | 'success' | 'warning' | 'danger' | 'neutral';
		size?: 'sm' | 'md';
		dot?: boolean;
		class?: string;
		children?: Snippet;
	}

	let {
		variant = 'neutral',
		size = 'md',
		dot = false,
		class: customClass = '',
		children
	}: Props = $props();

	let variantClasses = $derived(
		variant === 'primary'
			? 'bg-primary/10 dark:bg-primary/20 text-primary dark:text-primary-light border-primary/30'
			: variant === 'success'
				? 'bg-accent/10 dark:bg-accent/20 text-teal-700 dark:text-accent border-accent/30'
				: variant === 'warning'
					? 'bg-warning/10 dark:bg-warning/20 text-amber-700 dark:text-amber-300 border-warning/30'
					: variant === 'danger'
						? 'bg-danger/10 dark:bg-danger/20 text-rose-700 dark:text-rose-300 border-danger/30'
						: 'bg-slate-500/10 dark:bg-slate-500/20 text-slate-700 dark:text-slate-300 border-slate-500/20'
	);

	let dotColor = $derived(
		variant === 'primary'
			? 'bg-primary'
			: variant === 'success'
				? 'bg-accent'
				: variant === 'warning'
					? 'bg-warning'
					: variant === 'danger'
						? 'bg-danger'
						: 'bg-slate-400'
	);
</script>

<span
	class="inline-flex items-center font-medium border rounded-full {size === 'sm' ? 'px-2 py-0.5 text-2xs' : 'px-2.5 py-1 text-xs'} {variantClasses} {customClass}"
>
	{#if dot}
		<span class="w-1.5 h-1.5 rounded-full mr-1.5 {dotColor}"></span>
	{/if}
	{@render children?.()}
</span>

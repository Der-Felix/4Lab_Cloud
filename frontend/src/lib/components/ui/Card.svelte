<script lang="ts">
	import type { Snippet } from 'svelte';

	interface Props {
		title?: string;
		description?: string;
		class?: string;
		headerClass?: string;
		bodyClass?: string;
		footerClass?: string;
		header?: Snippet;
		children?: Snippet;
		footer?: Snippet;
	}

	let {
		title,
		description,
		class: customClass = '',
		headerClass = '',
		bodyClass = '',
		footerClass = '',
		header,
		children,
		footer
	}: Props = $props();
</script>

<div class="rounded-xl bg-surface-light dark:bg-surface-dark border border-border-light dark:border-border-dark card-depth shadow-depth shadow-depth-hover {customClass}">
	{#if header || title}
		<div class="px-5 py-3.5 border-b border-border-light dark:border-border-dark flex items-center justify-between {headerClass}">
			{#if header}
				{@render header()}
			{:else}
				<div>
					{#if title}
						<h3 class="font-semibold text-sm sm:text-base text-text-light dark:text-text-dark leading-none">{title}</h3>
					{/if}
					{#if description}
						<p class="text-xs text-muted-light dark:text-muted-dark mt-1">{description}</p>
					{/if}
				</div>
			{/if}
		</div>
	{/if}

	<div class="p-5 {bodyClass}">
		{@render children?.()}
	</div>

	{#if footer}
		<div class="px-5 py-3 border-t border-border-light dark:border-border-dark bg-slate-50/50 dark:bg-slate-900/30 rounded-b-xl flex items-center justify-end gap-2 {footerClass}">
			{@render footer()}
		</div>
	{/if}
</div>

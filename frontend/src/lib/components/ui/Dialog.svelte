<script lang="ts">
	import type { Snippet } from 'svelte';
	import { IconX } from '@tabler/icons-svelte';

	interface Props {
		open?: boolean;
		title?: string;
		description?: string;
		size?: 'sm' | 'md' | 'lg' | 'xl';
		class?: string;
		children?: Snippet;
		footer?: Snippet;
		onclose?: () => void;
	}

	let {
		open = $bindable(false),
		title,
		description,
		size = 'md',
		class: customClass = '',
		children,
		footer,
		onclose
	}: Props = $props();

	function close() {
		open = false;
		onclose?.();
	}

	function handleKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && open) {
			close();
		}
	}

	let sizeClasses = $derived(
		size === 'sm'
			? 'max-w-md'
			: size === 'lg'
				? 'max-w-2xl'
				: size === 'xl'
					? 'max-w-4xl'
					: 'max-w-lg'
	);
</script>

<svelte:window onkeydown={handleKeydown} />

{#if open}
	<div
		class="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-6"
		role="dialog"
		aria-modal="true"
	>
		<!-- Backdrop -->
		<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
		<div
			class="fixed inset-0 bg-black/60 backdrop-blur-xs transition-opacity animate-in fade-in duration-200"
			onclick={close}
		></div>

		<!-- Dialog Panel -->
		<div
			class="relative w-full {sizeClasses} rounded-xl bg-surface-light dark:bg-surface-dark border border-border-light dark:border-border-dark shadow-xl overflow-hidden z-10 animate-in zoom-in-95 duration-150 {customClass}"
		>
			{#if title}
				<div class="px-6 py-4 border-b border-border-light dark:border-border-dark flex items-center justify-between">
					<div>
						<h3 class="font-semibold text-base sm:text-lg text-text-light dark:text-text-dark leading-none">{title}</h3>
						{#if description}
							<p class="text-xs text-muted-light dark:text-muted-dark mt-1.5">{description}</p>
						{/if}
					</div>
					<button
						type="button"
						class="rounded-lg p-1.5 text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
						onclick={close}
						aria-label="Schließen"
					>
						<IconX size={18} stroke={1.75} />
					</button>
				</div>
			{/if}

			<div class="p-6 max-h-[80vh] overflow-y-auto">
				{@render children?.()}
			</div>

			{#if footer}
				<div class="px-6 py-3.5 border-t border-border-light dark:border-border-dark bg-slate-50/50 dark:bg-slate-900/40 flex items-center justify-end gap-2.5">
					{@render footer()}
				</div>
			{/if}
		</div>
	</div>
{/if}

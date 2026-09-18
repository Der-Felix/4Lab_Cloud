<script lang="ts">
	import { toasts, removeToast } from '$lib/stores';
	import { IconCircleCheck, IconAlertCircle, IconInfoCircle, IconX } from '@tabler/icons-svelte';
</script>

<div class="fixed top-4 right-4 z-50 flex flex-col gap-2 max-w-sm w-full pointer-events-none p-2 sm:p-0">
	{#each $toasts as toast (toast.id)}
		<div
			class="pointer-events-auto flex items-start gap-3 p-3.5 rounded-xl border shadow-lg backdrop-blur-md transition-all duration-200 animate-in slide-in-from-top-2 {
				toast.type === 'success'
					? 'bg-teal-950/90 text-teal-100 border-accent/40'
					: toast.type === 'error'
						? 'bg-rose-950/90 text-rose-100 border-danger/40'
						: 'bg-surface-dark/95 text-slate-100 border-border-dark'
			}"
			role="alert"
		>
			<div class="shrink-0 mt-0.5">
				{#if toast.type === 'success'}
					<IconCircleCheck size={16} stroke={1.75} class="text-accent" />
				{:else}
					{#if toast.type === 'error'}
						<IconAlertCircle size={16} stroke={1.75} class="text-danger" />
					{:else}
						<IconInfoCircle size={16} stroke={1.75} class="text-primary-light" />
					{/if}
				{/if}
			</div>

			<div class="flex-1 text-xs sm:text-sm font-medium leading-snug">
				{toast.message}
			</div>

			<button
				type="button"
				class="shrink-0 text-slate-400 hover:text-white rounded-lg p-0.5 transition-colors cursor-pointer"
				onclick={() => removeToast(toast.id)}
				aria-label="Benachrichtigung schließen"
			>
				<IconX size={16} stroke={1.75} />
			</button>
		</div>
	{/each}
</div>

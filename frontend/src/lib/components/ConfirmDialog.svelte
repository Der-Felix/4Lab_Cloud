<script lang="ts">
	import { Dialog, Button } from '$lib/components/ui';
	import { IconAlertTriangle, IconTrash } from '@tabler/icons-svelte';

	interface Props {
		open?: boolean;
		title?: string;
		message?: string;
		confirmText?: string;
		cancelText?: string;
		danger?: boolean;
		onconfirm: () => void;
		oncancel: () => void;
	}

	let {
		open = false,
		title = 'Bestätigung erforderlich',
		message = 'Sind Sie sicher?',
		confirmText = 'Löschen',
		cancelText = 'Abbrechen',
		danger = true,
		onconfirm,
		oncancel
	}: Props = $props();
</script>

<Dialog {open} onclose={oncancel} size="sm">
	<div class="flex items-start gap-3.5">
		<div class="p-2.5 rounded-xl {danger ? 'bg-danger/10 text-danger' : 'bg-primary/10 text-primary dark:text-primary-light'} shrink-0">
			{#if danger}
				<IconTrash size={20} stroke={1.75} />
			{:else}
				<IconAlertTriangle size={20} stroke={1.75} />
			{/if}
		</div>
		<div>
			<h3 class="text-base font-semibold text-text-light dark:text-text-dark">{title}</h3>
			<p class="text-xs text-muted-light dark:text-muted-dark mt-1.5 leading-relaxed">{message}</p>
		</div>
	</div>

	{#snippet footer()}
		<Button variant="secondary" size="sm" onclick={oncancel}>
			{cancelText}
		</Button>
		<Button variant={danger ? 'danger' : 'primary'} size="sm" onclick={onconfirm}>
			{confirmText}
		</Button>
	{/snippet}
</Dialog>

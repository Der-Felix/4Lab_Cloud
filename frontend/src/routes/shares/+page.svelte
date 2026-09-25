<script lang="ts">
	import { onMount } from 'svelte';
	import { listShares, deleteShare, type ShareItem } from '$lib/shares';
	import { formatBytes, formatDate } from '$lib/files';
	import { Button, Badge } from '$lib/components/ui';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
	import { addToast } from '$lib/stores';
	import {
		Share2,
		Copy,
		Check,
		Lock,
		Globe,
		ExternalLink,
		Trash,
		FileText,
		FolderClosed,
		RefreshCw
	} from '@lucide/svelte';

	let shares = $state<ShareItem[]>([]);
	let isLoading = $state(true);
	let filter = $state('');
	let copiedId = $state<string | null>(null);

	// Dialog-Zustand
	let shareToDelete = $state<ShareItem | null>(null);
	let showDeleteDialog = $state(false);

	async function loadShares() {
		isLoading = true;
		try {
			shares = await listShares();
		} catch (err) {
			console.error('Fehler beim Laden der Freigaben', err);
			addToast('Freigaben konnten nicht geladen werden', 'error');
		} finally {
			isLoading = false;
		}
	}

	onMount(() => {
		loadShares();
	});

	let filteredShares = $derived(
		filter.trim()
			? shares.filter((s) => s.filename.toLowerCase().includes(filter.toLowerCase()))
			: shares
	);

	async function copyLink(share: ShareItem) {
		const fullUrl = `${window.location.origin}${share.share_url}`;
		await navigator.clipboard.writeText(fullUrl);
		copiedId = share.id;
		addToast('Freigabelink kopiert', 'info');
		setTimeout(() => {
			if (copiedId === share.id) copiedId = null;
		}, 2000);
	}

	function promptDelete(share: ShareItem) {
		shareToDelete = share;
		showDeleteDialog = true;
	}

	async function confirmDelete() {
		if (!shareToDelete) return;
		try {
			await deleteShare(shareToDelete.id);
			addToast(`Freigabe für "${shareToDelete.filename}" widerrufen`, 'success');
			showDeleteDialog = false;
			shareToDelete = null;
			await loadShares();
		} catch (err) {
			addToast('Fehler beim Widerrufen der Freigabe', 'error');
		}
	}

	function isExpired(expiresAt: string | null): boolean {
		if (!expiresAt) return false;
		return new Date(expiresAt).getTime() < Date.now();
	}
</script>

<div class="space-y-6">
	<!-- Seiten-Header -->
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b border-border-light dark:border-border-dark">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-text-light dark:text-text-dark">
				Freigaben
			</h1>
			<p class="text-xs sm:text-sm text-muted-light dark:text-muted-dark mt-1">
				Verwalten Sie Ihre aktiven Freigabelinks, Passwörter und Gültigkeitsfristen.
			</p>
		</div>

		<div class="flex items-center gap-2.5">
			<Button href="/files" variant="primary" size="sm">
				<FolderClosed class="w-4 h-4 mr-1.5" />
				<span>Zu den Dateien</span>
			</Button>
		</div>
	</div>

	<!-- Suchleiste & Refresh -->
	<div class="flex items-center justify-between gap-3">
		<div class="relative flex-1 max-w-sm">
			<input
				type="text"
				placeholder="Freigaben durchsuchen..."
				bind:value={filter}
				class="w-full px-3.5 py-1.5 text-xs rounded-lg border border-border-light dark:border-border-dark bg-surface-light dark:bg-surface-dark text-text-light dark:text-text-dark placeholder-muted-light dark:placeholder-muted-dark focus:outline-hidden focus:border-primary transition-all"
			/>
		</div>

		<Button
			variant="secondary"
			size="sm"
			onclick={loadShares}
			title="Aktualisieren"
			aria-label="Aktualisieren"
		>
			<RefreshCw class="w-3.5 h-3.5 {isLoading ? 'animate-spin' : ''}" />
		</Button>
	</div>

	<!-- Freigaben-Tabelle -->
	<div class="rounded-xl bg-surface-light dark:bg-surface-dark border border-border-light dark:border-border-dark shadow-soft overflow-hidden">
		<div class="overflow-x-auto">
			<table class="w-full text-left border-collapse">
				<thead>
					<tr class="border-b border-border-light dark:border-border-dark bg-slate-50/75 dark:bg-slate-900/40 text-xs font-semibold text-muted-light dark:text-muted-dark">
						<th class="py-3 px-4">Datei</th>
						<th class="py-3 px-4">Schutz</th>
						<th class="py-3 px-4 hidden sm:table-cell">Gültig bis</th>
						<th class="py-3 px-4 hidden md:table-cell">Erstellt am</th>
						<th class="py-3 px-4 text-right">Aktionen</th>
					</tr>
				</thead>

				<tbody class="divide-y divide-border-light dark:divide-border-dark text-xs">
					{#if isLoading}
						{#each Array(4) as _}
							<tr>
								<td class="py-3 px-4"><div class="h-4 w-44 bg-slate-200 dark:bg-slate-800 animate-skeleton rounded"></div></td>
								<td class="py-3 px-4"><div class="h-4 w-16 bg-slate-200 dark:bg-slate-800 animate-skeleton rounded"></div></td>
								<td class="py-3 px-4 hidden sm:table-cell"><div class="h-4 w-20 bg-slate-200 dark:bg-slate-800 animate-skeleton rounded"></div></td>
								<td class="py-3 px-4 hidden md:table-cell"><div class="h-4 w-20 bg-slate-200 dark:bg-slate-800 animate-skeleton rounded"></div></td>
								<td class="py-3 px-4 text-right"><div class="h-4 w-16 ml-auto bg-slate-200 dark:bg-slate-800 animate-skeleton rounded"></div></td>
							</tr>
						{/each}
					{:else if filteredShares.length === 0}
						<tr>
							<td colspan="5" class="py-16 text-center">
								<div class="flex flex-col items-center justify-center space-y-2">
									<div class="w-10 h-10 rounded-full bg-slate-100 dark:bg-slate-800 flex items-center justify-center text-muted-light dark:text-muted-dark">
										<Share2 class="w-5 h-5" />
									</div>
									<p class="text-xs font-medium text-text-light dark:text-text-dark">Keine aktiven Freigaben gefunden</p>
									<p class="text-2xs text-muted-light dark:text-muted-dark">
										Klicken Sie in der Dateiansicht auf das Teilen-Symbol einer Datei, um einen Freigabelink zu erstellen.
									</p>
								</div>
							</td>
						</tr>
					{:else}
						{#each filteredShares as share (share.id)}
							{@const expired = isExpired(share.expires_at)}
							<tr class="hover:bg-slate-50 dark:hover:bg-slate-800/40 transition-colors">
								<!-- Dateiname & Groesse -->
								<td class="py-3 px-4">
									<div class="flex items-center gap-2.5 min-w-0">
										<div class="p-1.5 rounded-lg bg-primary/10 text-primary dark:text-primary-light shrink-0">
											<FileText class="w-4 h-4" />
										</div>
										<div class="min-w-0">
											<p class="font-medium text-text-light dark:text-text-dark truncate max-w-[200px] sm:max-w-xs" title={share.filename}>
												{share.filename}
											</p>
											<p class="text-2xs text-muted-light dark:text-muted-dark tabular-nums">
												{formatBytes(share.size_bytes)}
											</p>
										</div>
									</div>
								</td>

								<!-- Schutz-Status -->
								<td class="py-3 px-4 whitespace-nowrap">
									{#if share.has_password}
										<Badge variant="warning" size="sm">
											<Lock class="w-3 h-3 mr-1" />
											Passwort
										</Badge>
									{:else}
										<Badge variant="neutral" size="sm">
											<Globe class="w-3 h-3 mr-1" />
											Öffentlich
										</Badge>
									{/if}
								</td>

								<!-- Gueltigkeit -->
								<td class="py-3 px-4 whitespace-nowrap hidden sm:table-cell">
									{#if expired}
										<Badge variant="danger" size="sm">Abgelaufen</Badge>
									{:else if share.expires_at}
										<span class="text-muted-light dark:text-muted-dark tabular-nums">
											{formatDate(share.expires_at)}
										</span>
									{:else}
										<span class="text-muted-light dark:text-muted-dark">Unbegrenzt</span>
									{/if}
								</td>

								<!-- Erstellt -->
								<td class="py-3 px-4 whitespace-nowrap hidden md:table-cell text-muted-light dark:text-muted-dark tabular-nums">
									{formatDate(share.created_at)}
								</td>

								<!-- Aktionen -->
								<td class="py-3 px-4 text-right whitespace-nowrap">
									<div class="flex items-center justify-end gap-1.5">
										<!-- Link kopieren -->
										<Button
											variant="secondary"
											size="sm"
											onclick={() => copyLink(share)}
											title="Link kopieren"
										>
											{#if copiedId === share.id}
												<Check class="w-3.5 h-3.5 text-accent mr-1" />
												<span>Kopiert</span>
											{:else}
												<Copy class="w-3.5 h-3.5 mr-1" />
												<span>Kopieren</span>
											{/if}
										</Button>

										<!-- Im neuen Tab oeffnen -->
										<a
											href={share.share_url}
											target="_blank"
											rel="noopener noreferrer"
											class="p-1.5 rounded-lg text-muted-light dark:text-muted-dark hover:text-primary dark:hover:text-primary-light hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors"
											title="Freigabeseite öffnen"
										>
											<ExternalLink class="w-4 h-4" />
										</a>

										<!-- Widerrufen -->
										<button
											type="button"
											onclick={() => promptDelete(share)}
											class="p-1.5 rounded-lg text-muted-light dark:text-muted-dark hover:text-danger hover:bg-danger/10 transition-colors cursor-pointer"
											title="Freigabe widerrufen"
										>
											<Trash class="w-4 h-4" />
										</button>
									</div>
								</td>
							</tr>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>
	</div>
</div>

<!-- Loesch-Bestaetigung -->
<ConfirmDialog
	open={showDeleteDialog}
	title="Freigabe widerrufen?"
	message={`Möchten Sie den Freigabelink für "${shareToDelete?.filename}" wirklich deaktivieren? Niemand kann die Datei mehr über diesen Link herunterladen.`}
	confirmText="Freigabe widerrufen"
	danger={true}
	onconfirm={confirmDelete}
	oncancel={() => { showDeleteDialog = false; shareToDelete = null; }}
/>

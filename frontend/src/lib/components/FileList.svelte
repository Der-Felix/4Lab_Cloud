<script lang="ts">
	import FileRow from './FileRow.svelte';
	import type { FileItem } from '$lib/files';
	import { Button } from '$lib/components/ui';
	import { formatBytes, formatDate } from '$lib/files';
	import {
		IconArrowsSort,
		IconSortAscending,
		IconSortDescending,
		IconDownload,
		IconTrash,
		IconFolder,
		IconX,
		IconShare,
		IconEdit,
		IconFileText,
		IconPhoto,
		IconVideo,
		IconMusic,
		IconCode,
		IconFile
	} from '@tabler/icons-svelte';

	interface Props {
		files?: FileItem[];
		total?: number;
		page?: number;
		limit?: number;
		sort?: string;
		order?: 'asc' | 'desc';
		viewMode?: 'list' | 'grid';
		isLoading?: boolean;
		onsortchange: (field: string) => void;
		onpagechange: (newPage: number) => void;
		ondelete: (f: FileItem) => void;
		onbulkdelete?: (ids: string[]) => void;
		onrename: (f: FileItem, newName: string) => Promise<void>;
		ondownload: (f: FileItem) => void;
		onshare: (f: FileItem) => void;
	}

	let {
		files = [],
		total = 0,
		page = 1,
		limit = 50,
		sort = 'date',
		order = 'desc',
		viewMode = 'list',
		isLoading = false,
		onsortchange,
		onpagechange,
		ondelete,
		onbulkdelete,
		onrename,
		ondownload,
		onshare
	}: Props = $props();

	let selectedIds = $state<string[]>([]);
	let totalPages = $derived(Math.max(1, Math.ceil(total / limit)));

	let isAllSelected = $derived(
		files.length > 0 && files.every((f) => selectedIds.includes(f.id))
	);

	// Kontextmenue-Status
	let contextMenu = $state<{ file: FileItem; x: number; y: number } | null>(null);

	function toggleSelectAll() {
		if (isAllSelected) {
			selectedIds = [];
		} else {
			selectedIds = files.map((f) => f.id);
		}
	}

	function toggleItem(id: string) {
		if (selectedIds.includes(id)) {
			selectedIds = selectedIds.filter((i) => i !== id);
		} else {
			selectedIds = [...selectedIds, id];
		}
	}

	function clearSelection() {
		selectedIds = [];
	}

	function handleBulkDownload() {
		for (const id of selectedIds) {
			const f = files.find((item) => item.id === id);
			if (f) ondownload(f);
		}
	}

	function handleBulkDelete() {
		if (onbulkdelete) {
			onbulkdelete(selectedIds);
		} else {
			for (const id of selectedIds) {
				const f = files.find((item) => item.id === id);
				if (f) ondelete(f);
			}
		}
		clearSelection();
	}

	function handleContextMenu(e: MouseEvent, file: FileItem) {
		const x = Math.min(e.clientX, window.innerWidth - 180);
		const y = Math.min(e.clientY, window.innerHeight - 180);
		contextMenu = { file, x, y };
	}

	function closeContextMenu() {
		contextMenu = null;
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			closeContextMenu();
		}
	}

	function getFileIcon(mime: string) {
		if (mime.startsWith('image/')) return IconPhoto;
		if (mime.startsWith('video/')) return IconVideo;
		if (mime.startsWith('audio/')) return IconMusic;
		if (mime.includes('pdf') || mime.includes('text/')) return IconFileText;
		if (mime.includes('code') || mime.includes('json') || mime.includes('javascript')) return IconCode;
		return IconFile;
	}
</script>

<svelte:window onclick={closeContextMenu} onkeydown={handleKeydown} />

<div class="rounded-xl bg-surface-light dark:bg-surface-dark border border-border-light dark:border-border-dark shadow-soft overflow-hidden">
	<!-- Bulk-Action Toolbar wenn mindestens eine Datei ausgewaehlt ist -->
	{#if selectedIds.length > 0}
		<div class="px-4 py-2.5 bg-primary/10 border-b border-primary/25 flex items-center justify-between animate-in fade-in duration-150">
			<div class="flex items-center gap-2">
				<span class="text-xs font-semibold text-primary dark:text-primary-light">
					{selectedIds.length} {selectedIds.length === 1 ? 'Datei' : 'Dateien'} ausgewählt
				</span>
				<button
					type="button"
					onclick={clearSelection}
					class="p-1 rounded-md text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark cursor-pointer"
					title="Auswahl aufheben"
				>
					<IconX size={14} stroke={1.75} />
				</button>
			</div>

			<div class="flex items-center gap-2">
				<Button variant="secondary" size="sm" onclick={handleBulkDownload}>
					<IconDownload size={14} stroke={1.75} class="mr-1" />
					<span>Herunterladen</span>
				</Button>
				<Button variant="danger" size="sm" onclick={handleBulkDelete}>
					<IconTrash size={14} stroke={1.75} class="mr-1" />
					<span>Löschen</span>
				</Button>
			</div>
		</div>
	{/if}

	{#if viewMode === 'grid'}
		<!-- Grid-Ansicht -->
		<div class="p-4">
			{#if isLoading}
				<div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3.5">
					{#each Array(8) as _}
						<div class="h-28 rounded-xl bg-slate-200/70 dark:bg-slate-800/60 animate-skeleton"></div>
					{/each}
				</div>
			{:else if files.length === 0}
				<div class="py-12 text-center flex flex-col items-center justify-center">
					<img src="/assets/illustrations/empty-files.svg" alt="Keine Dateien" class="w-48 h-36 mb-3 select-none" />
					<p class="text-sm font-semibold text-text-light dark:text-text-dark">Keine Dateien vorhanden</p>
					<p class="text-xs text-muted-light dark:text-muted-dark mt-1">Ziehen Sie neue Dateien in das Browserfenster oder klicken Sie auf Hochladen.</p>
				</div>
			{:else}
				<div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-3.5">
					{#each files as file (file.id)}
						{@const IconComponent = getFileIcon(file.mime_type)}
						<div
							role="group"
							aria-label={file.filename}
							class="group relative p-3 rounded-xl border border-border-light dark:border-border-dark bg-surface-light dark:bg-surface-dark hover:border-primary/50 transition-all flex flex-col justify-between cursor-pointer {selectedIds.includes(file.id) ? 'ring-2 ring-primary' : ''}"
							oncontextmenu={(e) => { e.preventDefault(); handleContextMenu(e, file); }}
						>
							<div class="flex items-start justify-between">
								<div class="p-2 rounded-lg bg-primary/10 text-primary dark:text-primary-light">
									<IconComponent class="w-5 h-5" />
								</div>
								<input
									type="checkbox"
									checked={selectedIds.includes(file.id)}
									onchange={() => toggleItem(file.id)}
									class="w-4 h-4 rounded border-border-light dark:border-border-dark text-primary focus:ring-primary cursor-pointer"
								/>
							</div>

							<div class="mt-3">
								<button
									type="button"
									onclick={() => ondownload(file)}
									class="text-left font-medium text-xs text-text-light dark:text-text-dark hover:text-primary dark:hover:text-primary-light truncate block w-full"
									title={file.filename}
								>
									{file.filename}
								</button>
								<p class="text-[11px] text-muted-light dark:text-muted-dark mt-0.5 tabular-nums">
									{formatBytes(file.size_bytes)}
								</p>
							</div>
						</div>
					{/each}
				</div>
			{/if}
		</div>
	{:else}
		<!-- Tabellen-Ansicht -->
		<div class="overflow-x-auto">
			<table class="w-full text-left border-collapse">
				<thead>
					<tr class="border-b border-border-light dark:border-border-dark bg-slate-50/75 dark:bg-slate-900/40 text-xs font-semibold text-muted-light dark:text-muted-dark">
						<!-- Alle auswaehlen Checkbox -->
						<th class="py-2.5 pl-4 pr-2 w-10">
							<input
								type="checkbox"
								checked={isAllSelected}
								onchange={toggleSelectAll}
								class="w-4 h-4 rounded border-border-light dark:border-border-dark text-primary focus:ring-primary bg-surface-light dark:bg-surface-dark cursor-pointer"
								aria-label="Alle Dateien auswählen"
							/>
						</th>

						<!-- Name Header mit Sortierung -->
						<th class="py-2.5 px-3">
							<button
								type="button"
								onclick={() => onsortchange('name')}
								class="flex items-center gap-1 hover:text-text-light dark:hover:text-text-dark transition-colors cursor-pointer"
							>
								<span>Name</span>
								{#if sort === 'name'}
									{#if order === 'asc'}
										<IconSortAscending size={14} stroke={1.75} class="text-primary dark:text-primary-light" />
									{:else}
										<IconSortDescending size={14} stroke={1.75} class="text-primary dark:text-primary-light" />
									{/if}
								{:else}
									<IconArrowsSort size={14} stroke={1.75} class="opacity-30" />
								{/if}
							</button>
						</th>

						<!-- Groesse Header mit Sortierung -->
						<th class="py-2.5 px-3">
							<button
								type="button"
								onclick={() => onsortchange('size')}
								class="flex items-center gap-1 hover:text-text-light dark:hover:text-text-dark transition-colors cursor-pointer"
							>
								<span>Größe</span>
								{#if sort === 'size'}
									{#if order === 'asc'}
										<IconSortAscending size={14} stroke={1.75} class="text-primary dark:text-primary-light" />
									{:else}
										<IconSortDescending size={14} stroke={1.75} class="text-primary dark:text-primary-light" />
									{/if}
								{:else}
									<IconArrowsSort size={14} stroke={1.75} class="opacity-30" />
								{/if}
							</button>
						</th>

						<!-- Typ Header -->
						<th class="py-2.5 px-3 hidden md:table-cell">Typ</th>

						<!-- Datum Header mit Sortierung -->
						<th class="py-2.5 px-3 hidden sm:table-cell">
							<button
								type="button"
								onclick={() => onsortchange('date')}
								class="flex items-center gap-1 hover:text-text-light dark:hover:text-text-dark transition-colors cursor-pointer"
							>
								<span>Datum</span>
								{#if sort === 'date'}
									{#if order === 'asc'}
										<IconSortAscending size={14} stroke={1.75} class="text-primary dark:text-primary-light" />
									{:else}
										<IconSortDescending size={14} stroke={1.75} class="text-primary dark:text-primary-light" />
									{/if}
								{:else}
									<IconArrowsSort size={14} stroke={1.75} class="opacity-30" />
								{/if}
							</button>
						</th>

						<!-- Aktionen Header -->
						<th class="py-2.5 pr-4 pl-2 text-right">Aktionen</th>
					</tr>
				</thead>

				<tbody class="divide-y divide-border-light dark:divide-border-dark">
					{#if isLoading}
						{#each Array(6) as _}
							<tr>
								<td class="py-3 pl-4 pr-2"><div class="w-4 h-4 bg-slate-200 dark:bg-slate-800 animate-skeleton rounded"></div></td>
								<td class="py-3 px-3"><div class="h-4 w-44 bg-slate-200 dark:bg-slate-800 animate-skeleton rounded"></div></td>
								<td class="py-3 px-3"><div class="h-4 w-16 bg-slate-200 dark:bg-slate-800 animate-skeleton rounded"></div></td>
								<td class="py-3 px-3 hidden md:table-cell"><div class="h-4 w-12 bg-slate-200 dark:bg-slate-800 animate-skeleton rounded"></div></td>
								<td class="py-3 px-3 hidden sm:table-cell"><div class="h-4 w-20 bg-slate-200 dark:bg-slate-800 animate-skeleton rounded"></div></td>
								<td class="py-3 pr-4 pl-2 text-right"><div class="h-4 w-16 ml-auto bg-slate-200 dark:bg-slate-800 animate-skeleton rounded"></div></td>
							</tr>
						{/each}
					{:else if files.length === 0}
						<tr>
							<td colspan="6" class="py-12 text-center">
								<div class="flex flex-col items-center justify-center">
									<img src="/assets/illustrations/empty-files.svg" alt="Keine Dateien" class="w-48 h-36 mb-3 select-none" />
									<p class="text-sm font-semibold text-text-light dark:text-text-dark">Keine Dateien vorhanden</p>
									<p class="text-xs text-muted-light dark:text-muted-dark mt-1">Ziehen Sie neue Dateien in das Browserfenster oder klicken Sie auf Hochladen.</p>
								</div>
							</td>
						</tr>
					{:else}
						{#each files as file (file.id)}
							<FileRow
								{file}
								selected={selectedIds.includes(file.id)}
								ontoggle={toggleItem}
								{ondelete}
								{onrename}
								{ondownload}
								{onshare}
								oncontext={handleContextMenu}
							/>
						{/each}
					{/if}
				</tbody>
			</table>
		</div>
	{/if}

	<!-- Paginierung -->
	{#if total > 0}
		<div class="px-4 py-3 border-t border-border-light dark:border-border-dark bg-slate-50/50 dark:bg-slate-900/30 flex items-center justify-between text-xs text-muted-light dark:text-muted-dark">
			<div>
				Gesamt: <span class="font-semibold text-text-light dark:text-text-dark tabular-nums">{total}</span> Dateien
			</div>

			<div class="flex items-center gap-3">
				<span class="tabular-nums">Seite {page} von {totalPages}</span>
				<div class="flex items-center gap-1.5">
					<Button
						variant="secondary"
						size="sm"
						disabled={page <= 1}
						onclick={() => onpagechange(page - 1)}
					>
						Zurück
					</Button>
					<Button
						variant="secondary"
						size="sm"
						disabled={page >= totalPages}
						onclick={() => onpagechange(page + 1)}
					>
						Weiter
					</Button>
				</div>
			</div>
		</div>
	{/if}
</div>

<!-- Rechtsklick-Kontextmenue -->
{#if contextMenu}
	<div
		class="fixed z-50 w-48 rounded-xl bg-surface-light dark:bg-surface-dark border border-border-light dark:border-border-dark shadow-xl py-1.5 animate-in fade-in zoom-in-95 duration-100"
		style="left: {contextMenu.x}px; top: {contextMenu.y}px;"
		role="menu"
		tabindex="-1"
	>
		<div class="px-3 py-1.5 border-b border-border-light dark:border-border-dark text-[11px] font-semibold text-muted-light dark:text-muted-dark truncate">
			{contextMenu.file.filename}
		</div>

		<button
			type="button"
			class="w-full flex items-center gap-2.5 px-3 py-1.5 text-xs text-text-light dark:text-text-dark hover:bg-slate-100 dark:hover:bg-slate-800/60 transition-colors text-left cursor-pointer"
			onclick={() => { ondownload(contextMenu!.file); closeContextMenu(); }}
		>
			<IconDownload size={14} stroke={1.75} class="text-primary" />
			<span>Herunterladen</span>
		</button>

		<button
			type="button"
			class="w-full flex items-center gap-2.5 px-3 py-1.5 text-xs text-text-light dark:text-text-dark hover:bg-slate-100 dark:hover:bg-slate-800/60 transition-colors text-left cursor-pointer"
			onclick={() => { onshare(contextMenu!.file); closeContextMenu(); }}
		>
			<IconShare size={14} stroke={1.75} class="text-accent" />
			<span>Freigeben</span>
		</button>

		<button
			type="button"
			class="w-full flex items-center gap-2.5 px-3 py-1.5 text-xs text-danger hover:bg-danger/10 transition-colors text-left cursor-pointer"
			onclick={() => { ondelete(contextMenu!.file); closeContextMenu(); }}
		>
			<IconTrash size={14} stroke={1.75} />
			<span>Löschen</span>
		</button>
	</div>
{/if}

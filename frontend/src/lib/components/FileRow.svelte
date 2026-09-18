<script lang="ts">
	import { formatBytes, formatDate, type FileItem } from '$lib/files';
	import {
		IconPhoto,
		IconVideo,
		IconMusic,
		IconFileText,
		IconArchive,
		IconCode,
		IconFile,
		IconDownload,
		IconShare,
		IconEdit,
		IconTrash,
		IconCheck,
		IconX
	} from '@tabler/icons-svelte';

	interface Props {
		file: FileItem;
		selected?: boolean;
		ontoggle?: (id: string) => void;
		ondelete: (f: FileItem) => void;
		onrename: (f: FileItem, newName: string) => Promise<void>;
		ondownload: (f: FileItem) => void;
		onshare: (f: FileItem) => void;
		oncontext?: (e: MouseEvent, f: FileItem) => void;
	}

	let {
		file,
		selected = false,
		ontoggle,
		ondelete,
		onrename,
		ondownload,
		onshare,
		oncontext
	}: Props = $props();

	let isRenaming = $state(false);
	let editName = $state('');
	let isSaving = $state(false);

	function startRename() {
		editName = file.filename;
		isRenaming = true;
	}

	async function saveRename() {
		if (!editName.trim() || editName.trim() === file.filename) {
			isRenaming = false;
			return;
		}
		isSaving = true;
		try {
			await onrename(file, editName.trim());
			isRenaming = false;
		} finally {
			isSaving = false;
		}
	}

	function cancelRename() {
		isRenaming = false;
	}

	function handleKeydown(e: KeyboardEvent) {
		if (e.key === 'Escape') {
			cancelRename();
		}
	}

	function getFileInfo(mime: string) {
		if (mime.startsWith('image/')) {
			return { icon: IconPhoto, color: 'text-primary-light bg-primary-light/10' };
		}
		if (mime.startsWith('video/')) {
			return { icon: IconVideo, color: 'text-purple-400 bg-purple-500/10' };
		}
		if (mime.startsWith('audio/')) {
			return { icon: IconMusic, color: 'text-amber-400 bg-amber-500/10' };
		}
		if (mime.includes('pdf')) {
			return { icon: IconFileText, color: 'text-danger bg-danger/10' };
		}
		if (mime.includes('zip') || mime.includes('tar') || mime.includes('compressed') || mime.includes('gzip')) {
			return { icon: IconArchive, color: 'text-amber-500 bg-amber-500/10' };
		}
		if (mime.includes('javascript') || mime.includes('json') || mime.includes('html') || mime.includes('css') || mime.includes('code')) {
			return { icon: IconCode, color: 'text-accent bg-accent/10' };
		}
		return { icon: IconFile, color: 'text-muted-light dark:text-muted-dark bg-slate-500/10' };
	}

	let fileInfo = $derived(getFileInfo(file.mime_type));
	let IconComponent = $derived(fileInfo.icon);
</script>

<tr
	class="border-b border-border-light dark:border-border-dark transition-colors group {selected ? 'bg-primary/10' : 'hover:bg-slate-50 dark:hover:bg-slate-800/40'}"
	oncontextmenu={(e) => {
		e.preventDefault();
		oncontext?.(e, file);
	}}
>
	<!-- Checkbox zur Mehrfachauswahl -->
	<td class="py-2.5 pl-4 pr-2 w-10">
		<input
			type="checkbox"
			checked={selected}
			onchange={() => ontoggle?.(file.id)}
			class="w-4 h-4 rounded border-border-light dark:border-border-dark text-primary focus:ring-primary bg-surface-light dark:bg-surface-dark cursor-pointer"
			aria-label="Datei {file.filename} auswählen"
		/>
	</td>

	<!-- Dateiname & Icon -->
	<td class="py-2.5 px-3">
		<div class="flex items-center gap-3 min-w-0">
			<div class="w-8 h-8 rounded-lg flex items-center justify-center shrink-0 {fileInfo.color}">
				<IconComponent class="w-4 h-4" />
			</div>

			<div class="min-w-0 flex-1">
				{#if isRenaming}
					<form onsubmit={(e) => { e.preventDefault(); saveRename(); }} class="flex items-center gap-1.5">
						<input
							type="text"
							bind:value={editName}
							onkeydown={handleKeydown}
							class="px-2 py-1 text-xs rounded-lg border border-primary bg-surface-light dark:bg-bg-dark text-text-light dark:text-text-dark focus:outline-hidden"
							disabled={isSaving}
						/>
						<button
							type="submit"
							disabled={isSaving}
							class="px-2 py-1 rounded-md bg-primary text-white hover:bg-primary-hover flex items-center gap-1 text-xs font-medium cursor-pointer"
							title="Speichern"
						>
							<IconCheck size={14} stroke={1.75} />
							<span>Speichern</span>
						</button>
						<button
							type="button"
							onclick={cancelRename}
							class="px-2 py-1 rounded-md border border-border-light dark:border-border-dark text-muted-light dark:text-muted-dark hover:bg-slate-100 dark:hover:bg-slate-800 flex items-center gap-1 text-xs cursor-pointer"
							title="Abbrechen"
						>
							<IconX size={14} stroke={1.75} />
							<span>Abbrechen</span>
						</button>
					</form>
				{:else}
					<button
						type="button"
						ondblclick={startRename}
						onclick={() => ondownload(file)}
						class="text-left font-medium text-xs sm:text-sm text-text-light dark:text-text-dark hover:text-primary dark:hover:text-primary-light truncate block max-w-[220px] sm:max-w-xs md:max-w-sm transition-colors cursor-pointer"
						title={file.filename}
					>
						{file.filename}
					</button>
				{/if}
			</div>
		</div>
	</td>

	<!-- Groesse -->
	<td class="py-2.5 px-3 text-xs text-muted-light dark:text-muted-dark whitespace-nowrap tabular-nums">
		{formatBytes(file.size_bytes)}
	</td>

	<!-- Typ -->
	<td class="py-2.5 px-3 text-xs text-muted-light dark:text-muted-dark hidden md:table-cell max-w-[120px] truncate" title={file.mime_type}>
		{file.mime_type.split('/')[1] || file.mime_type}
	</td>

	<!-- Datum -->
	<td class="py-2.5 px-3 text-xs text-muted-light dark:text-muted-dark hidden sm:table-cell whitespace-nowrap tabular-nums">
		{formatDate(file.created_at)}
	</td>

	<!-- Aktionen -->
	<td class="py-2.5 pr-4 pl-2 text-right whitespace-nowrap">
		<div class="flex items-center justify-end gap-1 opacity-80 sm:opacity-0 group-hover:opacity-100 transition-opacity">
			<button
				type="button"
				onclick={() => ondownload(file)}
				class="p-1.5 rounded-lg text-muted-light dark:text-muted-dark hover:text-primary dark:hover:text-primary-light hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
				title="Herunterladen"
				aria-label="Herunterladen"
			>
				<IconDownload size={14} stroke={1.75} />
			</button>

			<button
				type="button"
				onclick={() => onshare(file)}
				class="p-1.5 rounded-lg text-muted-light dark:text-muted-dark hover:text-accent dark:hover:text-accent hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
				title="Freigeben"
				aria-label="Freigeben"
			>
				<IconShare size={14} stroke={1.75} />
			</button>

			<button
				type="button"
				onclick={startRename}
				class="p-1.5 rounded-lg text-muted-light dark:text-muted-dark hover:text-warning hover:bg-slate-100 dark:hover:bg-slate-800 transition-colors cursor-pointer"
				title="Umbenennen"
				aria-label="Umbenennen"
			>
				<IconEdit size={14} stroke={1.75} />
			</button>

			<button
				type="button"
				onclick={() => ondelete(file)}
				class="p-1.5 rounded-lg text-muted-light dark:text-muted-dark hover:text-danger hover:bg-danger/10 transition-colors cursor-pointer"
				title="Löschen"
				aria-label="Löschen"
			>
				<IconTrash size={14} stroke={1.75} />
			</button>
		</div>
	</td>
</tr>

<script lang="ts">
	import { onMount } from 'svelte';
	import { currentUser, addToast } from '$lib/stores';
	import {
		listFiles,
		formatBytes,
		downloadFile,
		deleteFile,
		getQuota,
		formatRelativeTime,
		type FileItem,
		type QuotaResponse
	} from '$lib/files';
	import { listShares, type ShareItem } from '$lib/shares';
	import { getMyAuditLogs, type AuditLogItem } from '$lib/admin';
	import Card from '$lib/components/ui/Card.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import FileCard from '$lib/components/dashboard/FileCard.svelte';
	import MapPreview from '$lib/components/dashboard/MapPreview.svelte';
	import ShareDialog from '$lib/components/ShareDialog.svelte';
	import ConfirmDialog from '$lib/components/ConfirmDialog.svelte';
	import {
		Upload,
		Image,
		ArrowRight,
		Share2,
		HardDrive,
		ShieldCheck,
		CheckCircle2,
		Lock,
		Globe,
		ExternalLink,
		Activity,
		FileText,
		FileSpreadsheet,
		FileVideo,
		FileAudio,
		File as FileIcon
	} from '@lucide/svelte';

	let recentFiles = $state<FileItem[]>([]);
	let shares = $state<ShareItem[]>([]);
	let activityLogs = $state<AuditLogItem[]>([]);
	let quota = $state<QuotaResponse>({
		used_bytes: 0,
		total_bytes: 50 * 1024 * 1024 * 1024,
		percent: 0
	});
	let totalFiles = $state<number>(0);
	let isLoading = $state<boolean>(true);
	let activeTab = $state<'all' | 'files' | 'images' | 'documents'>('all');

	// Dialog-Zustaende
	let fileToShare = $state<FileItem | null>(null);
	let showShareDialog = $state(false);
	let fileToDelete = $state<FileItem | null>(null);
	let showDeleteDialog = $state(false);

	// Benutzername fuer Hero-Begruessung
	let userName = $derived.by(() => {
		if (!$currentUser?.email) return 'Freund';
		const localPart = $currentUser.email.split('@')[0];
		return localPart.charAt(0).toUpperCase() + localPart.slice(1);
	});

	// Prozentuale Speicheranzeige
	let displayPercent = $derived.by(() => {
		if (quota.used_bytes > 0 && quota.percent < 0.1) {
			return '< 0.1%';
		}
		return `${quota.percent}%`;
	});

	// Filter fuer "Zuletzt verwendet" (Tabs: Alle, Dateien, Bilder, Dokumente)
	let filteredFiles = $derived.by(() => {
		if (activeTab === 'all') return recentFiles;
		if (activeTab === 'images') {
			return recentFiles.filter((f) => f.mime_type?.startsWith('image/'));
		}
		if (activeTab === 'documents') {
			return recentFiles.filter((f) => {
				const m = f.mime_type?.toLowerCase() || '';
				return (
					m.includes('pdf') ||
					m.includes('document') ||
					m.includes('word') ||
					m.includes('sheet') ||
					m.includes('excel') ||
					m.includes('text/') ||
					m.includes('presentation')
				);
			});
		}
		if (activeTab === 'files') {
			return recentFiles.filter((f) => !f.mime_type?.startsWith('image/'));
		}
		return recentFiles;
	});

	onMount(async () => {
		await loadDashboardData();
	});

	async function loadDashboardData() {
		try {
			const [filesRes, sharesRes, quotaRes, auditRes] = await Promise.allSettled([
				listFiles(1, 16, 'date', 'desc'),
				listShares(),
				getQuota(),
				getMyAuditLogs()
			]);

			if (filesRes.status === 'fulfilled') {
				recentFiles = filesRes.value.files || [];
				totalFiles = filesRes.value.total || 0;
			}

			if (sharesRes.status === 'fulfilled') {
				shares = sharesRes.value || [];
			}

			if (quotaRes.status === 'fulfilled') {
				quota = quotaRes.value;
			}

			if (auditRes.status === 'fulfilled') {
				activityLogs = auditRes.value || [];
			}
		} catch (err) {
			console.error('Fehler beim Laden der Dashboard-Daten', err);
		} finally {
			isLoading = false;
		}
	}

	async function handleDownload(file: FileItem) {
		try {
			const res = await downloadFile(file.id);
			if (res.download_url) {
				const a = document.createElement('a');
				a.href = res.download_url;
				a.download = res.filename || file.filename;
				document.body.appendChild(a);
				a.click();
				document.body.removeChild(a);
				addToast(`Download für "${file.filename}" gestartet`, 'success');
			}
		} catch {
			addToast('Download konnte nicht gestartet werden', 'error');
		}
	}

	function handleShare(file: FileItem) {
		fileToShare = file;
		showShareDialog = true;
	}

	function handleDeletePrompt(file: FileItem) {
		fileToDelete = file;
		showDeleteDialog = true;
	}

	async function confirmDelete() {
		if (!fileToDelete) return;
		try {
			await deleteFile(fileToDelete.id);
			recentFiles = recentFiles.filter((f) => f.id !== fileToDelete!.id);
			totalFiles = Math.max(0, totalFiles - 1);
			addToast(`"${fileToDelete.filename}" wurde gelöscht`, 'success');
			getQuota().then((q) => (quota = q));
		} catch {
			addToast('Datei konnte nicht gelöscht werden', 'error');
		} finally {
			showDeleteDialog = false;
			fileToDelete = null;
		}
	}

	function formatAuditAction(action: string): string {
		switch (action) {
			case 'file_upload':
				return 'Datei hochgeladen';
			case 'file_download':
				return 'Datei heruntergeladen';
			case 'file_delete':
				return 'Datei gelöscht';
			case 'file_rename':
				return 'Datei umbenannt';
			case 'share_create':
				return 'Freigabe erstellt';
			case 'share_delete':
				return 'Freigabe widerrufen';
			case 'auth_login':
			case 'login':
				return 'Erfolgreich angemeldet';
			case 'auth_logout':
			case 'logout':
				return 'Abgemeldet';
			case 'export_request':
				return 'Datenexport beantragt';
			case 'export_download':
				return 'Datenexport heruntergeladen';
			default:
				return action.replace(/_/g, ' ');
		}
	}

	// Farbe des Bullet-Points nach Aktionstyp (upload=teal, share=amber, login=slate, delete=rose)
	function getAuditDotColor(action: string): string {
		switch (action) {
			case 'file_upload':
				return 'bg-accent';
			case 'share_create':
				return 'bg-amber';
			case 'auth_login':
			case 'login':
			case 'auth_logout':
			case 'logout':
				return 'bg-slate';
			case 'file_delete':
			case 'share_delete':
				return 'bg-rose';
			default:
				return 'bg-primary';
		}
	}

	// Ermittelt Kategorie-Icon und -Farbe fuer Share-Eintraege
	function getShareFileInfo(filename: string) {
		const ext = filename.split('.').pop()?.toLowerCase() || '';
		if (['jpg', 'jpeg', 'png', 'webp', 'gif', 'svg'].includes(ext)) {
			return { icon: Image, color: 'text-accent' };
		}
		if (['pdf', 'doc', 'docx', 'txt', 'rtf'].includes(ext)) {
			return { icon: FileText, color: 'text-amber' };
		}
		if (['xls', 'xlsx', 'csv'].includes(ext)) {
			return { icon: FileSpreadsheet, color: 'text-amber' };
		}
		if (['mp4', 'mov', 'webm', 'mkv', 'avi'].includes(ext)) {
			return { icon: FileVideo, color: 'text-rose' };
		}
		if (['mp3', 'wav', 'flac', 'ogg', 'm4a'].includes(ext)) {
			return { icon: FileAudio, color: 'text-violet' };
		}
		return { icon: FileIcon, color: 'text-slate' };
	}
</script>

<div class="space-y-6">
	<!-- 1. Hero-Banner (280px hoch, echtes hero-default.jpg als object-cover mit rgba(0,0,0,0.4) Overlay) -->
	<div class="relative overflow-hidden rounded-2xl border border-border-light dark:border-border-dark h-[280px] flex items-center justify-between p-6 sm:p-8 card-depth shadow-depth">
		<!-- Hintergrundfoto mit dunklem Overlay fuer Barrierefreiheit -->
		<img
			src="/assets/hero-default.jpg"
			alt="4LabCloud Bergpanorama"
			class="absolute inset-0 w-full h-full object-cover select-none pointer-events-none"
		/>
		<div class="absolute inset-0 bg-black/40"></div>

		<!-- Links: Begruessung & 2 Buttons mit Backdrop-Blur -->
		<div class="relative z-10 max-w-lg backdrop-blur-md bg-black/25 p-5 sm:p-6 rounded-xl border border-white/15 shadow-depth">
			<h1 class="text-2xl sm:text-[30px] font-bold tracking-tight text-white leading-tight drop-shadow-sm">
				Schön, dich wieder zu sehen, {userName}!
			</h1>
			<p class="mt-1.5 text-sm sm:text-base text-white/85 leading-normal">
				Deine Dateien, Fotos und Projekte an einem sicheren Ort.
			</p>

			<!-- 2 Quick-Action-Buttons -->
			<div class="mt-4 sm:mt-5 flex items-center gap-3">
				<Button href="/files/upload" variant="primary" size="md">
					<Upload class="w-4 h-4 mr-2" />
					<span>Dateien hochladen</span>
				</Button>

				<Button
					href="/photos"
					variant="secondary"
					size="md"
					class="bg-white/15 hover:bg-white/25 text-white border-white/20 backdrop-blur-xs transition-colors"
				>
					<Image class="w-4 h-4 mr-2 text-accent" />
					<span>Fotos öffnen</span>
				</Button>
			</div>
		</div>

		<!-- Rechts: Prominenter Slogan in Great Vibes mit weicher Schattierung -->
		<div class="hidden md:flex flex-col items-end justify-center relative z-10 pr-4 select-none pointer-events-none">
			<span class="font-calligraphic text-3xl lg:text-4xl text-white tracking-wide block drop-shadow-[0_2px_8px_rgba(0,0,0,0.8)]">
				Deine Daten. Deine Freiheit.
			</span>
			<span class="text-[11px] font-sans tracking-widest uppercase text-white/80 mt-1 block drop-shadow-xs">
				Private Cloud Platform
			</span>
		</div>
	</div>

	<!-- 2. Hauptbereich: Breiteres 2-Spalten-Layout (Flexibel + 320px Sidebar) -->
	<div class="flex flex-col lg:flex-row gap-6 items-start">
		<!-- Linke Spalte: "Zuletzt verwendet" mit Tabs & dynamischem Kachel-Grid -->
		<div class="flex-1 min-w-0 space-y-4">
			<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-1 border-b border-border-light dark:border-border-dark">
				<!-- Section-Header mit 2x16px Akzentbalken (Primary) -->
				<div class="flex items-center gap-2.5">
					<div class="w-0.5 h-4 rounded-full bg-primary shrink-0"></div>
					<h2 class="text-lg font-semibold text-text-light dark:text-text-dark">
						Zuletzt verwendet
					</h2>
				</div>

				<!-- Filter-Tabs: Alle, Dateien, Bilder, Dokumente -->
				<div class="flex items-center gap-1 p-0.5 rounded-lg bg-slate-100 dark:bg-bg-dark border border-border-light dark:border-border-dark text-xs card-depth">
					<button
						type="button"
						onclick={() => (activeTab = 'all')}
						class="px-2.5 py-1 rounded-md transition-colors cursor-pointer {activeTab === 'all' ? 'bg-surface-light dark:bg-surface-dark text-text-light dark:text-text-dark font-medium shadow-xs' : 'text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark'}"
					>
						Alle
					</button>
					<button
						type="button"
						onclick={() => (activeTab = 'files')}
						class="px-2.5 py-1 rounded-md transition-colors cursor-pointer {activeTab === 'files' ? 'bg-surface-light dark:bg-surface-dark text-text-light dark:text-text-dark font-medium shadow-xs' : 'text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark'}"
					>
						Dateien
					</button>
					<button
						type="button"
						onclick={() => (activeTab = 'images')}
						class="px-2.5 py-1 rounded-md transition-colors cursor-pointer {activeTab === 'images' ? 'bg-surface-light dark:bg-surface-dark text-text-light dark:text-text-dark font-medium shadow-xs' : 'text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark'}"
					>
						Bilder
					</button>
					<button
						type="button"
						onclick={() => (activeTab = 'documents')}
						class="px-2.5 py-1 rounded-md transition-colors cursor-pointer {activeTab === 'documents' ? 'bg-surface-light dark:bg-surface-dark text-text-light dark:text-text-dark font-medium shadow-xs' : 'text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark'}"
					>
						Dokumente
					</button>
				</div>
			</div>

			<!-- Dynamisches Kachel-Grid (4 Spalten, 5 bei xl) -->
			{#if isLoading}
				<div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
					{#each Array(10) as _}
						<div class="aspect-4/5 rounded-xl bg-slate-200/70 dark:bg-slate-800/60 animate-skeleton"></div>
					{/each}
				</div>
			{:else if filteredFiles.length === 0}
				<!-- Empty-State mit Illustration: nur sichtbar wenn Liste leer ist -->
				<div class="p-8 rounded-xl border border-border-light dark:border-border-dark bg-surface-light dark:bg-surface-dark text-center flex flex-col items-center justify-center card-depth shadow-depth">
					<img src="/assets/illustrations/empty-files.svg" alt="Keine Dateien" class="w-48 h-36 mb-3 select-none" />
					<p class="text-sm font-semibold text-text-light dark:text-text-dark">
						{activeTab === 'all' ? 'Keine Dateien vorhanden' : 'In dieser Kategorie sind keine Dateien vorhanden'}
					</p>
					<p class="text-xs text-muted-light dark:text-muted-dark mt-1 mb-4">
						Laden Sie Ihre erste Datei hoch.
					</p>
					<Button href="/files/upload" variant="primary" size="sm">
						<Upload class="w-3.5 h-3.5 mr-1.5" />
						<span>Dateien hochladen</span>
					</Button>
				</div>
			{:else}
				<div class="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4">
					{#each filteredFiles.slice(0, 10) as file (file.id)}
						<FileCard
							{file}
							ondownload={handleDownload}
							onshare={handleShare}
							ondelete={handleDeletePrompt}
						/>
					{/each}
				</div>

				<div class="pt-2 text-right">
					<a
						href="/files"
						class="text-xs font-medium text-primary dark:text-primary-light hover:underline inline-flex items-center gap-1"
					>
						<span>Alle {totalFiles} Dateien anzeigen</span>
						<ArrowRight class="w-3.5 h-3.5" />
					</a>
				</div>
			{/if}
		</div>

		<!-- Rechte Spalte: Sidebar-Karten (w-80 / 320px) mit Tiefe & Akzenten -->
		<div class="w-full lg:w-80 shrink-0 space-y-6">
			<!-- Karte 1: Ihre Freigaben (mit Akzentbalken Amber & Kategorie-Icons) -->
			<Card class="p-5">
				<div class="flex items-center justify-between pb-3 border-b border-border-light dark:border-border-dark">
					<div class="flex items-center gap-2.5">
						<div class="w-0.5 h-4 rounded-full bg-amber shrink-0"></div>
						<div class="p-1.5 rounded-lg bg-amber/15 text-amber">
							<Share2 class="w-4 h-4" />
						</div>
						<h3 class="font-semibold text-sm text-text-light dark:text-text-dark">
							Ihre Freigaben
						</h3>
					</div>
					<Badge variant="neutral" size="sm">
						<span class="tabular-nums">{shares.length}</span>
					</Badge>
				</div>

				<div class="mt-4">
					{#if isLoading}
						<div class="space-y-2.5">
							{#each Array(3) as _}
								<div class="h-12 bg-slate-200/70 dark:bg-slate-800/60 animate-skeleton rounded-lg"></div>
							{/each}
						</div>
					{:else if shares.length === 0}
						<!-- Empty-State mit Illustration -->
						<div class="py-6 text-center text-xs text-muted-light dark:text-muted-dark flex flex-col items-center justify-center">
							<img src="/assets/illustrations/empty-shares.svg" alt="Keine Freigaben" class="w-36 h-28 mb-2 select-none opacity-80" />
							<p class="font-medium text-text-light dark:text-text-dark">Noch keine Freigaben</p>
							<p class="mt-1 text-[11px]">Geben Sie Dateien über das Kontextmenü frei.</p>
						</div>
					{:else}
						<div class="space-y-2.5 divide-y divide-border-light dark:divide-border-dark -my-1">
							{#each shares.slice(0, 4) as share}
								{@const fileInfo = getShareFileInfo(share.filename)}
								{@const FileTypeIcon = fileInfo.icon}
								<div class="pt-2.5 first:pt-0 flex items-center justify-between gap-3 text-xs">
									<div class="min-w-0 flex-1 flex items-center gap-2.5">
										<!-- Kategorie-farbiges Datei-Icon -->
										<div class="p-1 rounded-md bg-slate-100 dark:bg-bg-dark shrink-0 {fileInfo.color}">
											<FileTypeIcon class="w-3.5 h-3.5" />
										</div>

										<div class="min-w-0 flex-1">
											<div class="flex items-center gap-1.5">
												{#if share.has_password}
													<Lock class="w-3 h-3 text-amber shrink-0" title="Passwortgeschützt" />
												{:else}
													<Globe class="w-3 h-3 text-accent shrink-0" title="Öffentlich" />
												{/if}
												<span class="font-medium text-text-light dark:text-text-dark truncate" title={share.filename}>
													{share.filename}
												</span>
											</div>
											<div class="mt-0.5 text-[11px] text-muted-light dark:text-muted-dark flex items-center gap-1.5">
												{#if share.has_password}
													<span class="text-amber">Passwortgeschützt</span>
												{:else}
													<span class="text-accent">Öffentlich</span>
												{/if}
												<span>·</span>
												<span class="tabular-nums">{formatBytes(share.size_bytes)}</span>
											</div>
										</div>
									</div>
									<a
										href="/shares"
										class="p-1 rounded-md text-muted-light dark:text-muted-dark hover:text-primary hover:bg-slate-100 dark:hover:bg-bg-dark transition-colors shrink-0"
										title="Freigabe ansehen"
									>
										<ExternalLink class="w-3.5 h-3.5" />
									</a>
								</div>
							{/each}
						</div>
					{/if}

					<div class="mt-4 pt-3 border-t border-border-light dark:border-border-dark">
						<a
							href="/shares"
							class="text-xs font-medium text-primary dark:text-primary-light hover:underline inline-flex items-center justify-between w-full"
						>
							<span>Alle Freigaben verwalten</span>
							<ArrowRight class="w-3.5 h-3.5" />
						</a>
					</div>
				</div>
			</Card>

			<!-- Karte 2: Speicherplatz (mit Akzentbalken Primary & korrekten Quota-Zahlen) -->
			<Card class="p-5">
				<div class="flex items-center justify-between pb-3 border-b border-border-light dark:border-border-dark">
					<div class="flex items-center gap-2.5">
						<div class="w-0.5 h-4 rounded-full bg-primary shrink-0"></div>
						<div class="p-1.5 rounded-lg bg-accent/15 text-accent">
							<HardDrive class="w-4 h-4" />
						</div>
						<h3 class="font-semibold text-sm text-text-light dark:text-text-dark">
							Speicherplatz
						</h3>
					</div>
					<span class="text-xs font-semibold tabular-nums text-text-light dark:text-text-dark">
						{displayPercent}
					</span>
				</div>

				<div class="mt-4 space-y-3">
					<!-- Fortschrittsbalken mit primary-Farbe -->
					<div class="w-full h-2 rounded-full bg-slate-100 dark:bg-bg-dark border border-border-light dark:border-border-dark overflow-hidden">
						<div
							class="h-full bg-primary transition-all duration-300 rounded-full"
							style="width: {Math.max(quota.used_bytes > 0 ? 1 : 0, Math.min(100, quota.percent))}%"
						></div>
					</div>

					<div class="flex items-center justify-between text-xs text-muted-light dark:text-muted-dark tabular-nums">
						<span>{formatBytes(quota.used_bytes)} belegt</span>
						<span>von {formatBytes(quota.total_bytes)}</span>
					</div>
				</div>
			</Card>

			<!-- Karte 3: Aktivitaet (mit Akzentbalken Accent & farbigen Bullet-Points nach Aktionstyp) -->
			<Card class="p-5">
				<div class="flex items-center justify-between pb-3 border-b border-border-light dark:border-border-dark">
					<div class="flex items-center gap-2.5">
						<div class="w-0.5 h-4 rounded-full bg-accent shrink-0"></div>
						<div class="p-1.5 rounded-lg bg-primary/15 text-primary dark:text-primary-light">
							<Activity class="w-4 h-4" />
						</div>
						<h3 class="font-semibold text-sm text-text-light dark:text-text-dark">
							Aktivität
						</h3>
					</div>
					<Badge variant="neutral" size="sm">
						<span class="tabular-nums">{activityLogs.length}</span>
					</Badge>
				</div>

				<div class="mt-4">
					{#if isLoading}
						<div class="space-y-2.5">
							{#each Array(3) as _}
								<div class="h-10 bg-slate-200/70 dark:bg-slate-800/60 animate-skeleton rounded-lg"></div>
							{/each}
						</div>
					{:else if activityLogs.length === 0}
						<!-- Empty-State mit Illustration -->
						<div class="py-6 text-center text-xs text-muted-light dark:text-muted-dark flex flex-col items-center justify-center">
							<img src="/assets/illustrations/empty-activity.svg" alt="Keine Aktivität" class="w-36 h-28 mb-2 select-none opacity-80" />
							<p class="font-medium text-text-light dark:text-text-dark">Noch keine Aktivitäten</p>
							<p class="mt-1 text-[11px]">Hier erscheinen Ihre Datei-Operationen.</p>
						</div>
					{:else}
						<div class="space-y-3">
							{#each activityLogs.slice(0, 3) as log (log.id)}
								<div class="flex items-start gap-2.5 text-xs">
									<!-- Farbcodierter Bullet-Punkt: upload=teal, share=amber, login=slate, delete=rose -->
									<div class="mt-1 w-2 h-2 rounded-full shrink-0 {getAuditDotColor(log.action)}"></div>
									<div class="min-w-0 flex-1">
										<p class="font-medium text-text-light dark:text-text-dark truncate">
											{formatAuditAction(log.action)}
										</p>
										<p class="text-[11px] text-muted-light dark:text-muted-dark tabular-nums mt-0.5">
											{formatRelativeTime(log.created_at)}
										</p>
									</div>
								</div>
							{/each}
						</div>
					{/if}
				</div>
			</Card>

			<!-- Karte 4: Fotokarte Vorschau (nur wenn GPS-Fotos vorhanden) -->
			<MapPreview />

			<!-- Sicherheitsstatus (DSGVO & BSI) -->
			<div class="p-4 rounded-xl bg-surface-light dark:bg-surface-dark border border-border-light dark:border-border-dark card-depth shadow-depth space-y-2.5">
				<div class="flex items-center gap-2 font-semibold text-xs text-text-light dark:text-text-dark">
					<ShieldCheck class="w-4 h-4 text-accent" />
					<span>BSI TR-02102-2 & DSGVO konform</span>
				</div>
				<div class="space-y-1.5 text-[11px] text-muted-light dark:text-muted-dark">
					<div class="flex items-center gap-1.5">
						<CheckCircle2 class="w-3.5 h-3.5 text-accent shrink-0" />
						<span>AES-256-GCM at rest</span>
					</div>
					<div class="flex items-center gap-1.5">
						<CheckCircle2 class="w-3.5 h-3.5 text-accent shrink-0" />
						<span>TLS 1.3 in transit</span>
					</div>
					<div class="flex items-center gap-1.5">
						<CheckCircle2 class="w-3.5 h-3.5 text-accent shrink-0" />
						<span>RLS isoliert & Audit-Log aktiv</span>
					</div>
				</div>
			</div>
		</div>
	</div>
</div>

<!-- Dialoge -->
<ShareDialog
	file={fileToShare}
	open={showShareDialog}
	onclose={() => {
		showShareDialog = false;
		fileToShare = null;
		loadDashboardData();
	}}
/>

<ConfirmDialog
	open={showDeleteDialog}
	title="Datei löschen"
	message="Möchten Sie diese Datei wirklich unwiderruflich löschen? Sie wird vom Server entfernt."
	confirmText="Endgültig löschen"
	danger={true}
	onconfirm={confirmDelete}
	oncancel={() => {
		showDeleteDialog = false;
		fileToDelete = null;
	}}
/>

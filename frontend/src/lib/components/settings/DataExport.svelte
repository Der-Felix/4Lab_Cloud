<script lang="ts">
	import { onDestroy } from 'svelte';
	import { requestExport, getExportStatus, getExportDownloadUrl, type ExportJobStatus } from '$lib/exports';
	import { addToast } from '$lib/stores';
	import { Card, Button, Input, Dialog, Badge } from '$lib/components/ui';
	import {
		Download,
		FileArchive,
		Clock,
		CheckCircle2,
		AlertCircle,
		HelpCircle,
		Lock,
		RefreshCw,
		FileText
	} from '@lucide/svelte';

	let showPasswordDialog = $state(false);
	let showHelpDialog = $state(false);
	let exportPassword = $state('');
	let isSubmitting = $state(false);
	let exportJob = $state<ExportJobStatus | null>(null);
	let pollInterval: ReturnType<typeof setInterval> | null = null;

	onDestroy(() => {
		stopPolling();
	});

	function stopPolling() {
		if (pollInterval) {
			clearInterval(pollInterval);
			pollInterval = null;
		}
	}

	async function handleStartExport(e: SubmitEvent) {
		e.preventDefault();
		if (!exportPassword.trim()) return;

		isSubmitting = true;
		try {
			const res = await requestExport(exportPassword.trim());
			exportPassword = '';
			showPasswordDialog = false;
			addToast('Datenexport-Auftrag erfolgreich gestartet', 'success');

			exportJob = {
				job_id: res.job_id,
				status: 'pending'
			};
			startPolling(res.job_id);
		} catch (err: unknown) {
			addToast(err instanceof Error ? err.message : 'Export fehlgeschlagen', 'error');
		} finally {
			isSubmitting = false;
		}
	}

	function startPolling(jobId: string) {
		stopPolling();
		pollStatus(jobId);
		pollInterval = setInterval(() => pollStatus(jobId), 3000);
	}

	async function pollStatus(jobId: string) {
		try {
			const status = await getExportStatus(jobId);
			exportJob = status;

			if (status.status === 'completed') {
				stopPolling();
				addToast('Datenexport bereit zum Herunterladen!', 'success');
			} else if (status.status === 'failed' || status.status === 'expired') {
				stopPolling();
				addToast('Export-Auftrag fehlgeschlagen oder abgelaufen', 'error');
			}
		} catch (err) {
			console.error('Fehler beim Statusabruf des Exports:', err);
		}
	}

	function handleDownloadTriggered() {
		addToast('Datei mit Ihrem Passwort verschlüsselt', 'info', 6000);
	}
</script>

<Card
	title="Recht auf Datenübertragbarkeit (Art. 20 DSGVO)"
	description="Exportieren Sie alle Ihre gespeicherten Daten und Metadaten als passwortgeschützte ZIP-Datei"
>
	<div class="space-y-4 text-xs">
		<p class="text-muted-light dark:text-muted-dark leading-relaxed">
			Sie haben das Recht, alle Ihre in 4labscloud gespeicherten personenbezogenen Daten, Dateien und Freigabe-Metadaten
			in einem maschinenlesbaren, strukturierten Format zu erhalten.
		</p>

		<div class="p-4 rounded-xl bg-slate-50 dark:bg-bg-dark border border-border-light dark:border-border-dark space-y-3">
			<div class="flex items-start gap-3">
				<div class="p-2 rounded-lg bg-primary/10 text-primary dark:text-primary-light shrink-0">
					<FileArchive class="w-5 h-5" />
				</div>
				<div class="space-y-1">
					<h4 class="font-semibold text-text-light dark:text-text-dark">
						Passwortgeschütztes ZIP-Archiv
					</h4>
					<p class="text-muted-light dark:text-muted-dark text-[11px] leading-relaxed">
						Enthält alle Ihre Originaldateien, Metadaten im JSON-Format sowie eine ausführliche Entschlüsselungsanleitung
						(<code class="font-mono text-primary font-semibold">HOW_TO_DECRYPT.txt</code>).
					</p>
				</div>
			</div>

			<div class="flex flex-wrap items-center justify-between gap-3 pt-2 border-t border-border-light dark:border-border-dark">
				<div class="flex items-center gap-1.5 text-[11px] text-muted-light dark:text-muted-dark">
					<Clock class="w-3.5 h-3.5 text-primary" />
					<span>Bereitgestellte Exporte laufen nach <strong>7 Tagen</strong> automatisch ab.</span>
				</div>

				<button
					type="button"
					onclick={() => (showHelpDialog = true)}
					class="text-[11px] text-primary hover:underline flex items-center gap-1 cursor-pointer"
				>
					<HelpCircle class="w-3.5 h-3.5" />
					<span>Wie öffne ich die Datei?</span>
				</button>
			</div>
		</div>

		<!-- Action Trigger -->
		<div class="flex items-center gap-3 pt-2">
			<Button
				variant="primary"
				size="sm"
				onclick={() => (showPasswordDialog = true)}
				disabled={exportJob?.status === 'pending' || exportJob?.status === 'processing'}
			>
				<Download class="w-3.5 h-3.5 mr-1.5" />
				<span>Export anfordern</span>
			</Button>

			{#if exportJob}
				<div class="flex items-center gap-2">
					{#if exportJob.status === 'pending' || exportJob.status === 'processing'}
						<Badge variant="primary" size="sm" dot={true}>
							<RefreshCw class="w-3 h-3 mr-1 animate-spin" />
							<span>Wird vorbereitet...</span>
						</Badge>
					{:else if exportJob.status === 'completed'}
						<Badge variant="success" size="sm">
							<CheckCircle2 class="w-3 h-3 mr-1 text-accent" />
							<span>Bereit</span>
						</Badge>
					{:else if exportJob.status === 'failed'}
						<Badge variant="danger" size="sm">
							<AlertCircle class="w-3 h-3 mr-1" />
							<span>Fehlgeschlagen</span>
						</Badge>
					{/if}
				</div>
			{/if}
		</div>

		<!-- Job Result Card -->
		{#if exportJob && exportJob.status === 'completed'}
			<div class="p-3.5 rounded-xl bg-accent/10 border border-accent/20 flex items-center justify-between gap-4">
				<div class="flex items-center gap-2.5">
					<CheckCircle2 class="w-4 h-4 text-accent shrink-0" />
					<div>
						<p class="font-medium text-text-light dark:text-text-dark">Ihr Datenexport ist abholbereit</p>
						<p class="text-[11px] text-muted-light dark:text-muted-dark">
							Verschlüsselt mit dem von Ihnen gewählten Passwort.
						</p>
					</div>
				</div>

				<Button
					href={exportJob.download_url || getExportDownloadUrl(exportJob.job_id)}
					variant="primary"
					size="sm"
					onclick={handleDownloadTriggered}
				>
					<Download class="w-3.5 h-3.5 mr-1.5" />
					<span>ZIP herunterladen</span>
				</Button>
			</div>
		{/if}
	</div>
</Card>

<!-- Modal: Kennwortabfrage fuer Export-Erstellung -->
<Dialog
	open={showPasswordDialog}
	onclose={() => (showPasswordDialog = false)}
	title="Datenexport anfordern (Art. 20 DSGVO)"
	size="md"
>
	<form onsubmit={handleStartExport} class="space-y-4 text-xs">
		<p class="text-muted-light dark:text-muted-dark leading-relaxed">
			Aus Sicherheitsgründen wird Ihr Datenexport mit AES-256 verschlüsselt. Bitte vergeben Sie ein Passwort,
			mit dem das Archiv nach dem Herunterladen geöffnet werden kann.
		</p>

		<Input
			id="export-pw"
			label="Passwort für das ZIP-Archiv"
			type="password"
			bind:value={exportPassword}
			required
			placeholder="Sicheres Archiv-Passwort"
		/>

		<div class="p-3 rounded-lg bg-slate-100 dark:bg-bg-dark border border-border-light dark:border-border-dark flex items-start gap-2 text-[11px] text-muted-light dark:text-muted-dark">
			<Lock class="w-3.5 h-3.5 text-primary shrink-0 mt-0.5" />
			<span>
				Merken Sie sich dieses Passwort gut. Ohne dieses Kennwort können die exportierten Dateien nicht wiederhergestellt werden.
			</span>
		</div>

		<div class="flex items-center justify-end gap-2 pt-2">
			<Button variant="secondary" size="sm" onclick={() => (showPasswordDialog = false)}>
				<span>Abbrechen</span>
			</Button>
			<Button
				type="submit"
				variant="primary"
				size="sm"
				loading={isSubmitting}
				disabled={!exportPassword.trim()}
			>
				<Download class="w-3.5 h-3.5 mr-1.5" />
				<span>Auftrag starten</span>
			</Button>
		</div>
	</form>
</Dialog>

<!-- Modal: Entschlüsselungsanleitung (Wie öffne ich die Datei?) -->
<Dialog
	open={showHelpDialog}
	onclose={() => (showHelpDialog = false)}
	title="Anleitung: Datenexport öffnen"
	size="md"
>
	<div class="space-y-3.5 text-xs text-muted-light dark:text-muted-dark leading-relaxed">
		<p class="text-text-light dark:text-text-dark font-medium">
			Ihr Archiv wird nach dem BSI-konformen Standard mit AES-256 verschlüsselt.
		</p>

		<div class="space-y-2">
			<h5 class="font-semibold text-text-light dark:text-text-dark flex items-center gap-1.5">
				<FileText class="w-3.5 h-3.5 text-primary" />
				<span>HOW_TO_DECRYPT.txt im Archiv</span>
			</h5>
			<p>
				In jedem Export-Archiv befindet sich eine Datei namens <code class="font-mono text-primary">HOW_TO_DECRYPT.txt</code> mit detaillierten
				Befehlen für alle gängigen Betriebssysteme.
			</p>
		</div>

		<div class="space-y-1.5">
			<h5 class="font-semibold text-text-light dark:text-text-dark">Unter Windows & macOS:</h5>
			<p>
				Verwenden Sie <strong>7-Zip</strong> (Windows), <strong>Keka</strong> oder <strong>The Unarchiver</strong> (macOS).
				Geben Sie beim Entpacken das bei der Anforderung vergebene Passwort ein.
			</p>
		</div>

		<div class="space-y-1.5">
			<h5 class="font-semibold text-text-light dark:text-text-dark">Im Terminal (Linux / macOS):</h5>
			<pre class="p-2.5 rounded-lg bg-slate-100 dark:bg-bg-dark border border-border-light dark:border-border-dark font-mono text-[11px] text-text-light dark:text-text-dark select-all">7z x export.zip -pDEIN_PASSWORT</pre>
		</div>
	</div>

	{#snippet footer()}
		<Button variant="secondary" size="sm" onclick={() => (showHelpDialog = false)}>
			<span>Schließen</span>
		</Button>
	{/snippet}
</Dialog>

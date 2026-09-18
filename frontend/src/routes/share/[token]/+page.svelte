<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import Logo from '$lib/components/Logo.svelte';
	import { formatBytes, formatDate } from '$lib/files';
	import { Download, AlertCircle, Lock } from '@lucide/svelte';

	interface ShareInfo {
		token: string;
		filename: string;
		size_bytes: number;
		mime_type: string;
		requires_password: boolean;
		expires_at?: string;
	}

	let share = $state<ShareInfo | null>(null);
	let isLoading = $state(true);
	let error = $state<string | null>(null);
	let password = $state('');
	let isDownloading = $state(false);
	let downloadError = $state<string | null>(null);

	const token = page.params.token;

	onMount(async () => {
		try {
			const res = await fetch(`/api/v1/shares/${token}`);
			if (!res.ok) {
				if (res.status === 410) {
					error = 'Dieser Freigabelink ist abgelaufen.';
				} else {
					error = 'Die Freigabe wurde nicht gefunden oder wurde gelöscht.';
				}
				return;
			}
			share = await res.json();
		} catch {
			error = 'Verbindungsfehler beim Laden der Freigabe.';
		} finally {
			isLoading = false;
		}
	});

	async function handleDownload(e: SubmitEvent) {
		e.preventDefault();
		if (isDownloading) return;
		downloadError = null;
		isDownloading = true;

		try {
			const query = password ? `?password=${encodeURIComponent(password)}` : '';
			const res = await fetch(`/api/v1/shares/${token}/download${query}`);
			if (!res.ok) {
				const data = await res.json().catch(() => ({}));
				downloadError = data.error || 'Fehler beim Herunterladen der Datei.';
				return;
			}

			const data = await res.json();
			if (data.download_url) {
				const a = document.createElement('a');
				a.href = data.download_url;
				a.download = data.filename || share?.filename || 'download';
				document.body.appendChild(a);
				a.click();
				document.body.removeChild(a);
			}
		} catch {
			downloadError = 'Netzwerkfehler beim Anfordern des Downloads.';
		} finally {
			isDownloading = false;
		}
	}
</script>

<svelte:head>
	<title>{share ? `${share.filename} - Freigabe` : 'Datei-Freigabe'} - 4labscloud</title>
</svelte:head>

<div class="w-full max-w-md rounded-2xl border border-border-light dark:border-border-dark bg-surface-light dark:bg-surface-dark p-8 shadow-xl text-center">
	<div class="flex justify-center mb-6">
		<Logo size={48} />
	</div>

	{#if isLoading}
		<div class="flex justify-center py-12 text-muted-light dark:text-muted-dark">
			<div class="h-8 w-8 animate-skeleton rounded-full bg-slate-200 dark:bg-slate-800"></div>
		</div>
	{:else if error}
		<div class="rounded-xl border border-danger/20 bg-danger/10 p-4 text-xs text-danger mb-6 flex items-start gap-2">
			<AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
			<span>{error}</span>
		</div>
		<a href="/" class="text-xs font-semibold text-primary dark:text-primary-light hover:underline">
			Zur Startseite
		</a>
	{:else if share}
		<div class="w-14 h-14 mx-auto rounded-2xl bg-primary/10 text-primary dark:text-primary-light flex items-center justify-center mb-4">
			<Download class="w-7 h-7" />
		</div>

		<h2 class="text-base font-bold text-text-light dark:text-text-dark break-all mb-1">
			{share.filename}
		</h2>
		<p class="text-xs text-muted-light dark:text-muted-dark mb-6 tabular-nums">
			{formatBytes(share.size_bytes)} • {share.mime_type}
			{#if share.expires_at}
				<br />Gültig bis: {formatDate(share.expires_at)}
			{/if}
		</p>

		{#if downloadError}
			<div class="rounded-xl border border-danger/20 bg-danger/10 p-3 text-xs text-danger mb-4 flex items-start gap-2">
				<AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
				<span>{downloadError}</span>
			</div>
		{/if}

		<form onsubmit={handleDownload} class="space-y-4">
			{#if share.requires_password}
				<div class="text-left">
					<label for="share-pwd" class="block text-xs font-medium text-text-light dark:text-text-dark mb-1">
						Passwort für Freigabe
					</label>
					<input
						id="share-pwd"
						type="password"
						bind:value={password}
						placeholder="Passwort eingeben..."
						class="w-full rounded-lg border border-border-light dark:border-border-dark bg-surface-light dark:bg-bg-dark px-3.5 py-2 text-xs text-text-light dark:text-text-dark placeholder-muted-light dark:placeholder-muted-dark focus:border-primary focus:outline-hidden transition-all"
						required
					/>
				</div>
			{/if}

			<button
				type="submit"
				disabled={isDownloading || (share.requires_password && !password)}
				class="w-full flex items-center justify-center py-2.5 px-4 rounded-xl bg-primary hover:bg-primary-hover text-xs font-semibold text-white shadow-xs transition-colors disabled:opacity-50 cursor-pointer active:scale-[0.98]"
			>
				{#if isDownloading}
					<div class="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent mr-2"></div>
					<span>Wird vorbereitet...</span>
				{:else}
					<Download class="w-4 h-4 mr-2" />
					<span>Datei herunterladen</span>
				{/if}
			</button>
		</form>
	{/if}
</div>

<script lang="ts">
	import { createShare, type FileItem } from '$lib/files';
	import { Dialog, Button, Input } from '$lib/components/ui';
	import { addToast } from '$lib/stores';
	import { Share2, Copy, Check, AlertCircle } from '@lucide/svelte';

	interface Props {
		file?: FileItem | null;
		open?: boolean;
		onclose: () => void;
	}

	let {
		file = null,
		open = false,
		onclose
	}: Props = $props();

	let expiresDays = $state(7);
	let password = $state('');
	let shareUrl = $state('');
	let isLoading = $state(false);
	let errorMessage = $state('');
	let copied = $state(false);

	$effect(() => {
		if (!open) {
			shareUrl = '';
			errorMessage = '';
			password = '';
			expiresDays = 7;
			copied = false;
		}
	});

	async function handleSubmit(e: SubmitEvent) {
		e.preventDefault();
		if (!file) return;

		errorMessage = '';
		isLoading = true;

		try {
			const res = await createShare(file.id, {
				expires_days: Number(expiresDays),
				password: password.trim() || undefined
			});
			const fullUrl = window.location.origin + res.share_url;
			shareUrl = fullUrl;
			addToast('Freigabelink erfolgreich erstellt', 'success');
		} catch (err: unknown) {
			errorMessage = err instanceof Error ? err.message : 'Fehler beim Erstellen des Freigabelinks.';
		} finally {
			isLoading = false;
		}
	}

	async function copyToClipboard() {
		if (shareUrl) {
			await navigator.clipboard.writeText(shareUrl);
			copied = true;
			addToast('Link in Zwischenablage kopiert', 'info');
			setTimeout(() => {
				copied = false;
			}, 2000);
		}
	}
</script>

<Dialog
	{open}
	{onclose}
	title="Datei freigeben"
	description={file ? file.filename : ''}
	size="md"
>
	{#if errorMessage}
		<div class="mb-4 p-3 rounded-lg bg-danger/10 border border-danger/20 text-danger text-xs font-medium flex items-start gap-2">
			<AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
			<span>{errorMessage}</span>
		</div>
	{/if}

	{#if shareUrl}
		<div class="space-y-4">
			<div class="p-3.5 rounded-xl bg-accent/10 border border-accent/30 text-teal-700 dark:text-accent text-xs font-medium flex items-center gap-2">
				<Share2 class="w-4 h-4 shrink-0 text-accent" />
				<span>Freigabelink wurde sicher generiert!</span>
			</div>

			<div>
				<label for="share-url-result" class="block text-xs font-medium text-muted-light dark:text-muted-dark mb-1.5">
					Öffentlicher Link
				</label>
				<div class="flex gap-2">
					<input
						id="share-url-result"
						type="text"
						readonly
						value={shareUrl}
						class="flex-1 px-3 py-2 text-xs rounded-lg border border-border-light dark:border-border-dark bg-slate-50 dark:bg-bg-dark text-text-light dark:text-text-dark font-mono select-all focus:outline-hidden"
					/>
					<Button
						variant="primary"
						size="sm"
						onclick={copyToClipboard}
					>
						{#if copied}
							<Check class="w-4 h-4 mr-1 text-accent" />
							<span>Kopiert</span>
						{:else}
							<Copy class="w-4 h-4 mr-1" />
							<span>Kopieren</span>
						{/if}
					</Button>
				</div>
			</div>
		</div>
	{:else}
		<form onsubmit={handleSubmit} class="space-y-4">
			<div>
				<label for="share-expires" class="block text-xs font-medium text-text-light dark:text-text-dark mb-1.5">
					Gültigkeitsdauer
				</label>
				<select
					id="share-expires"
					bind:value={expiresDays}
					class="w-full px-3 py-2 rounded-lg border border-border-light dark:border-border-dark bg-surface-light dark:bg-bg-dark text-text-light dark:text-text-dark text-xs focus:outline-hidden focus:border-primary transition-all"
				>
					<option value={1}>1 Tag (Höchste Sicherheit)</option>
					<option value={7}>7 Tage (Standard)</option>
					<option value={30}>30 Tage</option>
					<option value={90}>90 Tage</option>
				</select>
			</div>

			<div>
				<Input
					id="share-password"
					label="Passwortschutz (optional)"
					type="password"
					bind:value={password}
					placeholder="Leer lassen für schlüsselfreien Link"
					hint="Empfohlen bei sensiblen Dokumenten"
				/>
			</div>

			<div class="pt-2 flex justify-end gap-2">
				<Button variant="secondary" size="sm" onclick={onclose}>
					<span>Abbrechen</span>
				</Button>
				<Button type="submit" variant="primary" size="sm" loading={isLoading}>
					<span>Link generieren</span>
				</Button>
			</div>
		</form>
	{/if}

	{#if shareUrl}
		{#snippet footer()}
			<Button variant="secondary" size="sm" onclick={onclose}>
				<span>Schließen</span>
			</Button>
		{/snippet}
	{/if}
</Dialog>

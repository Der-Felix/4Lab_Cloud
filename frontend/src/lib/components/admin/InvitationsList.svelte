<script lang="ts">
	import { onMount } from 'svelte';
	import { listInvitations, inviteUser, type InvitationItem, type InviteResponse } from '$lib/admin';
	import { formatDate } from '$lib/files';
	import { addToast } from '$lib/stores';
	import { Card, Button, Input, Dialog, Badge } from '$lib/components/ui';
	import {
		UserPlus,
		Mail,
		Copy,
		CheckCircle2,
		Clock,
		RefreshCw,
		Send
	} from '@lucide/svelte';

	let invitations = $state<InvitationItem[]>([]);
	let isLoading = $state(true);

	// Einladungs-Dialog
	let showDialog = $state(false);
	let inviteEmail = $state('');
	let isInviting = $state(false);
	let createdInvite = $state<{ url: string; email: string; expires_at: string } | null>(null);
	let copiedLink = $state(false);

	onMount(async () => {
		await load();
	});

	export async function load(): Promise<void> {
		isLoading = true;
		try {
			invitations = await listInvitations();
		} catch {
			addToast('Einladungen konnten nicht geladen werden', 'error');
		} finally {
			isLoading = false;
		}
	}

	export function openInviteModal() {
		inviteEmail = '';
		createdInvite = null;
		copiedLink = false;
		showDialog = true;
	}

	async function handleCreateInvite(e: SubmitEvent) {
		e.preventDefault();
		if (!inviteEmail.trim()) return;

		isInviting = true;
		try {
			const res = await inviteUser(inviteEmail.trim().toLowerCase());
			const origin = typeof window !== 'undefined' ? window.location.origin : '';
			const fullUrl = `${origin}/register?token=${res.invitation_token}`;

			createdInvite = {
				url: fullUrl,
				email: res.email,
				expires_at: res.expires_at
			};
			inviteEmail = '';
			addToast('Einladung erfolgreich generiert', 'success');
			await load();
		} catch (err: unknown) {
			addToast(err instanceof Error ? err.message : 'Einladung fehlgeschlagen', 'error');
		} finally {
			isInviting = false;
		}
	}

	async function handleCopyLink() {
		if (!createdInvite?.url) return;
		await navigator.clipboard.writeText(createdInvite.url);
		copiedLink = true;
		addToast('Einladungslink in die Zwischenablage kopiert', 'info');
		setTimeout(() => {
			copiedLink = false;
		}, 2500);
	}
</script>

<Card title="Offene Einladungen" description="Verwalten Sie aktive Registrierungs-Einladungen für neue Mitglieder">
	{#snippet header()}
		<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 w-full">
			<div>
				<h3 class="font-semibold text-sm sm:text-base text-text-light dark:text-text-dark leading-none">
					Offene Einladungen
				</h3>
				<p class="text-xs text-muted-light dark:text-muted-dark mt-1">
					Nur mit gültigem Einladungstoken können neue Konten registriert werden
				</p>
			</div>

			<div class="flex items-center gap-2">
				<Button variant="primary" size="sm" onclick={openInviteModal}>
					<UserPlus class="w-3.5 h-3.5 mr-1.5" />
					<span>Einladung erstellen</span>
				</Button>
				<Button variant="secondary" size="sm" onclick={load} disabled={isLoading} title="Aktualisieren">
					<RefreshCw class="w-3.5 h-3.5 {isLoading ? 'animate-spin' : ''}" />
				</Button>
			</div>
		</div>
	{/snippet}

	<div class="space-y-4 text-xs">
		{#if isLoading && invitations.length === 0}
			<div class="py-8 text-center text-xs text-muted-light dark:text-muted-dark space-y-2">
				<RefreshCw class="w-5 h-5 mx-auto animate-spin text-primary" />
				<p>Einladungen werden geladen...</p>
			</div>
		{:else if invitations.length === 0}
			<div class="py-8 text-center text-xs text-muted-light dark:text-muted-dark">
				<Mail class="w-8 h-8 mx-auto text-muted-light dark:text-muted-dark mb-2 opacity-50" />
				<p>Aktuell liegen keine offenen Einladungen vor.</p>
			</div>
		{:else}
			<div class="overflow-x-auto -mx-4 sm:-mx-6">
				<table class="w-full text-left text-xs border-collapse">
					<thead>
						<tr class="border-b border-border-light dark:border-border-dark bg-slate-50/50 dark:bg-surface-dark/50 text-muted-light dark:text-muted-dark">
							<th class="py-2.5 px-4 sm:px-6 font-medium">E-Mail des Eingeladenen</th>
							<th class="py-2.5 px-4 font-medium">Erstellt am</th>
							<th class="py-2.5 px-4 font-medium">Gültig bis</th>
							<th class="py-2.5 px-4 sm:px-6 text-right font-medium">Status</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-border-light dark:divide-border-dark">
						{#each invitations as inv (inv.id)}
							<tr class="hover:bg-slate-50/70 dark:hover:bg-surface-dark/70 transition-colors">
								<td class="py-3 px-4 sm:px-6 font-medium text-text-light dark:text-text-dark">
									<div class="flex items-center gap-2">
										<Mail class="w-3.5 h-3.5 text-primary" />
										<span>{inv.email}</span>
									</div>
								</td>
								<td class="py-3 px-4 text-muted-light dark:text-muted-dark tabular-nums">
									{formatDate(inv.created_at)}
								</td>
								<td class="py-3 px-4 text-muted-light dark:text-muted-dark tabular-nums">
									{formatDate(inv.expires_at)}
								</td>
								<td class="py-3 px-4 sm:px-6 text-right">
									<Badge variant="neutral" size="sm">
										<Clock class="w-3 h-3 mr-1 text-primary" />
										<span>Offen</span>
									</Badge>
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</div>
</Card>

<!-- Modal: Neue Einladung erstellen -->
<Dialog
	open={showDialog}
	onclose={() => (showDialog = false)}
	title="Neue Einladung erstellen"
	size="md"
>
	{#if !createdInvite}
		<form onsubmit={handleCreateInvite} class="space-y-4 text-xs">
			<p class="text-muted-light dark:text-muted-dark leading-relaxed">
				Geben Sie die E-Mail-Adresse der Person ein, die Sie zu 4labscloud einladen möchten.
				Es wird ein sicherer, zeitlich befristeter Registrierungslink erzeugt.
			</p>

			<Input
				id="invite-email-modal"
				label="E-Mail-Adresse"
				type="email"
				bind:value={inviteEmail}
				required
				placeholder="kollege@4labs.local"
			/>

			<div class="flex items-center justify-end gap-2 pt-2">
				<Button variant="secondary" size="sm" onclick={() => (showDialog = false)} disabled={isInviting}>
					<span>Abbrechen</span>
				</Button>
				<Button
					type="submit"
					variant="primary"
					size="sm"
					loading={isInviting}
					disabled={!inviteEmail.trim()}
				>
					<Send class="w-3.5 h-3.5 mr-1.5" />
					<span>Einladung generieren</span>
				</Button>
			</div>
		</form>
	{:else}
		<!-- Einmalige Anzeige des generierten Links gemaess Anforderung -->
		<div class="space-y-4 text-xs">
			<div class="p-3.5 rounded-xl bg-accent/10 border border-accent/30 space-y-1 text-text-light dark:text-text-dark">
				<p class="font-semibold text-accent flex items-center gap-1.5">
					<CheckCircle2 class="w-4 h-4" />
					<span>Einladungslink erfolgreich erstellt</span>
				</p>
				<p class="text-2xs text-muted-light dark:text-muted-dark">
					Dieser Link wird aus Sicherheitsgründen nur <strong>einmal</strong> angezeigt. Bitte leiten Sie ihn direkt an
					<strong>{createdInvite.email}</strong> weiter.
				</p>
			</div>

			<div class="space-y-1.5">
				<label class="text-2xs font-medium text-muted-light dark:text-muted-dark" for="invite-url-field">
					Registrierungs-URL:
				</label>
				<div class="flex gap-2">
					<input
						id="invite-url-field"
						type="text"
						readonly
						value={createdInvite.url}
						class="flex-1 px-3 py-1.5 text-xs rounded-lg border border-border-light dark:border-border-dark bg-slate-50 dark:bg-bg-dark font-mono text-text-light dark:text-text-dark select-all"
					/>
					<Button variant="primary" size="sm" onclick={handleCopyLink}>
						{#if copiedLink}
							<CheckCircle2 class="w-3.5 h-3.5 mr-1 text-accent" />
							<span>Kopiert</span>
						{:else}
							<Copy class="w-3.5 h-3.5 mr-1" />
							<span>Kopieren</span>
						{/if}
					</Button>
				</div>
			</div>
		</div>

		{#snippet footer()}
			<Button variant="secondary" size="sm" onclick={() => (showDialog = false)}>
				<span>Fertig</span>
			</Button>
		{/snippet}
	{/if}
</Dialog>

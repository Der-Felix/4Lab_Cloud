<script lang="ts">
	import { onMount } from 'svelte';
	import { listUsers, deleteUser, type AdminUserItem } from '$lib/admin';
	import { currentUser, addToast } from '$lib/stores';
	import { formatDate } from '$lib/files';
	import { Card, Button, Input, Badge, Dialog } from '$lib/components/ui';
	import {
		Users,
		Search,
		ShieldCheck,
		ShieldAlert,
		Trash2,
		RefreshCw,
		AlertTriangle,
		UserPlus
	} from '@lucide/svelte';

	let { oninvite }: { oninvite?: () => void } = $props();

	let users = $state<AdminUserItem[]>([]);
	let searchQuery = $state('');
	let isLoading = $state(true);

	// Zweistufiger Loesch-Dialog
	let userToDelete = $state<AdminUserItem | null>(null);
	let confirmEmailInput = $state('');
	let isDeleting = $state(false);

	onMount(async () => {
		await load();
	});

	export async function load(): Promise<void> {
		isLoading = true;
		try {
			users = await listUsers();
		} catch {
			addToast('Benutzerliste konnte nicht geladen werden', 'error');
		} finally {
			isLoading = false;
		}
	}

	let filteredUsers = $derived(
		users.filter((u) => {
			if (!searchQuery.trim()) return true;
			const q = searchQuery.toLowerCase().trim();
			return u.email.toLowerCase().includes(q) || u.id.toLowerCase().includes(q);
		})
	);

	function openDeleteDialog(user: AdminUserItem) {
		if (user.id === $currentUser?.id) {
			addToast('Sie können sich nicht selbst als Administrator löschen', 'error');
			return;
		}
		userToDelete = user;
		confirmEmailInput = '';
	}

	async function handleExecuteDelete() {
		if (!userToDelete) return;

		// Strikte Pruefung: E-Mail muss exakt uebereinstimmen (Zweistufige Bestaetigung)
		if (confirmEmailInput.trim().toLowerCase() !== userToDelete.email.toLowerCase()) {
			addToast('Die eingegebene E-Mail-Adresse stimmt nicht überein', 'error');
			return;
		}

		isDeleting = true;
		try {
			await deleteUser(userToDelete.id);
			addToast(`Benutzer "${userToDelete.email}" wurde gelöscht`, 'success');
			userToDelete = null;
			await load();
		} catch (err: unknown) {
			addToast(err instanceof Error ? err.message : 'Fehler beim Löschen des Benutzers', 'error');
		} finally {
			isDeleting = false;
		}
	}
</script>

<Card title="Benutzerverwaltung" description="Übersicht aller im System registrierten Konten">
	{#snippet header()}
		<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 w-full">
			<div>
				<h3 class="font-semibold text-sm sm:text-base text-text-light dark:text-text-dark leading-none">
					Benutzerverwaltung
				</h3>
				<p class="text-xs text-muted-light dark:text-muted-dark mt-1">
					{users.length} {users.length === 1 ? 'registrierter Benutzer' : 'registrierte Benutzer'}
				</p>
			</div>

			<div class="flex items-center gap-2">
				{#if oninvite}
					<Button variant="primary" size="sm" onclick={oninvite}>
						<UserPlus class="w-3.5 h-3.5 mr-1.5" />
						<span>Benutzer einladen</span>
					</Button>
				{/if}
				<Button variant="secondary" size="sm" onclick={load} disabled={isLoading} title="Aktualisieren">
					<RefreshCw class="w-3.5 h-3.5 {isLoading ? 'animate-spin' : ''}" />
				</Button>
			</div>
		</div>
	{/snippet}

	<div class="space-y-4 text-xs">
		<!-- Suchfeld -->
		<div class="relative max-w-sm">
			<Search class="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-light dark:text-muted-dark pointer-events-none" />
			<input
				type="text"
				placeholder="Benutzer nach E-Mail suchen..."
				bind:value={searchQuery}
				class="w-full pl-9 pr-3 py-2 text-xs rounded-lg border border-border-light dark:border-border-dark bg-surface-light dark:bg-bg-dark text-text-light dark:text-text-dark placeholder:text-muted-light dark:placeholder:text-muted-dark focus:outline-hidden focus:border-primary focus:ring-1 focus:ring-primary transition-all"
			/>
		</div>

		<!-- Benutzertabelle -->
		{#if isLoading && users.length === 0}
			<div class="py-8 text-center text-xs text-muted-light dark:text-muted-dark space-y-2">
				<RefreshCw class="w-5 h-5 mx-auto animate-spin text-primary" />
				<p>Benutzerliste wird geladen...</p>
			</div>
		{:else if filteredUsers.length === 0}
			<div class="py-8 text-center text-xs text-muted-light dark:text-muted-dark">
				<Users class="w-8 h-8 mx-auto text-muted-light dark:text-muted-dark mb-2 opacity-50" />
				<p>{searchQuery ? 'Keine Benutzer für diese Suchanfrage gefunden.' : 'Keine Benutzer registriert.'}</p>
			</div>
		{:else}
			<div class="overflow-x-auto -mx-4 sm:-mx-6">
				<table class="w-full text-left text-xs border-collapse">
					<thead>
						<tr class="border-b border-border-light dark:border-border-dark bg-slate-50/50 dark:bg-surface-dark/50 text-muted-light dark:text-muted-dark">
							<th class="py-2.5 px-4 sm:px-6 font-medium">E-Mail-Adresse</th>
							<th class="py-2.5 px-4 font-medium">Rolle</th>
							<th class="py-2.5 px-4 font-medium">MFA-Schutz</th>
							<th class="py-2.5 px-4 font-medium">Registriert am</th>
							<th class="py-2.5 px-4 sm:px-6 text-right font-medium">Aktionen</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-border-light dark:divide-border-dark">
						{#each filteredUsers as user (user.id)}
							{@const isMe = user.id === $currentUser?.id}
							<tr class="hover:bg-slate-50/70 dark:hover:bg-surface-dark/70 transition-colors {isMe ? 'bg-primary/5' : ''}">
								<td class="py-3 px-4 sm:px-6">
									<div class="font-medium text-text-light dark:text-text-dark flex items-center gap-2">
										<span>{user.email}</span>
										{#if isMe}
											<Badge variant="primary" size="sm">Sie</Badge>
										{/if}
									</div>
									<div class="font-mono text-2xs text-muted-light dark:text-muted-dark tabular-nums">
										{user.id}
									</div>
								</td>

								<td class="py-3 px-4">
									{#if user.is_admin}
										<Badge variant="primary" size="sm">Administrator</Badge>
									{:else}
										<Badge variant="neutral" size="sm">Benutzer</Badge>
									{/if}
								</td>

								<td class="py-3 px-4">
									{#if user.mfa_enabled}
										<span class="inline-flex items-center gap-1 text-accent font-medium">
											<ShieldCheck class="w-3.5 h-3.5" />
											<span>Aktiv (2FA)</span>
										</span>
									{:else}
										<span class="inline-flex items-center gap-1 text-muted-light dark:text-muted-dark">
											<ShieldAlert class="w-3.5 h-3.5" />
											<span>Inaktiv</span>
										</span>
									{/if}
								</td>

								<td class="py-3 px-4 text-muted-light dark:text-muted-dark tabular-nums">
									{formatDate(user.created_at)}
								</td>

								<td class="py-3 px-4 sm:px-6 text-right">
									{#if isMe}
										<button
											type="button"
											disabled
											class="p-1 rounded-md text-muted-light/40 dark:text-muted-dark/40 cursor-not-allowed"
											title="Sie können sich nicht selbst löschen"
										>
											<Trash2 class="w-4 h-4" />
										</button>
									{:else}
										<button
											type="button"
											onclick={() => openDeleteDialog(user)}
											class="p-1.5 rounded-lg text-muted-light dark:text-muted-dark hover:text-danger hover:bg-danger/10 transition-colors cursor-pointer"
											title={`Benutzer ${user.email} löschen`}
											aria-label={`Benutzer ${user.email} löschen`}
										>
											<Trash2 class="w-4 h-4" />
										</button>
									{/if}
								</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		{/if}
	</div>
</Card>

<!-- Zweistufiger Loesch-Bestaetigungsdialog -->
<Dialog
	open={userToDelete !== null}
	onclose={() => (userToDelete = null)}
	title="Benutzer endgültig löschen?"
	size="md"
>
	{#if userToDelete}
		<div class="space-y-4 text-xs">
			<div class="p-3 rounded-lg bg-danger/10 border border-danger/20 text-danger flex items-start gap-2">
				<AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
				<span class="leading-relaxed">
					<strong>Warnung:</strong> Durch diesen Vorgang werden der Benutzer <strong>{userToDelete.email}</strong>,
					alle hochgeladenen Dateien sowie alle Freigaben physisch und unwiderruflich aus dem System gelöscht.
				</span>
			</div>

			<p class="text-muted-light dark:text-muted-dark">
				Geben Sie zur Bestätigung die E-Mail-Adresse des zu löschenden Benutzers ein (<code class="font-mono text-danger font-semibold select-all">{userToDelete.email}</code>):
			</p>

			<Input
				id="admin-del-confirm-email"
				label="E-Mail-Adresse zur Bestätigung"
				type="email"
				bind:value={confirmEmailInput}
				placeholder={userToDelete.email}
				required
			/>
		</div>
	{/if}

	{#snippet footer()}
		<Button variant="secondary" size="sm" onclick={() => (userToDelete = null)} disabled={isDeleting}>
			<span>Abbrechen</span>
		</Button>
		<Button
			variant="danger"
			size="sm"
			loading={isDeleting}
			disabled={!userToDelete || confirmEmailInput.trim().toLowerCase() !== userToDelete.email.toLowerCase()}
			onclick={handleExecuteDelete}
		>
			<Trash2 class="w-3.5 h-3.5 mr-1.5" />
			<span>Endgültig löschen</span>
		</Button>
	{/snippet}
</Dialog>

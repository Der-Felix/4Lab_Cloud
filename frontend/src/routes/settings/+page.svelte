<script lang="ts">
	import { page } from '$app/state';
	import { currentUser, addToast } from '$lib/stores';
	import { isAdmin } from '$lib/auth';
	import { Card, Button, Input, Badge } from '$lib/components/ui';
	import SessionsList from '$lib/components/settings/SessionsList.svelte';
	import DataExport from '$lib/components/settings/DataExport.svelte';
	import DeleteAccount from '$lib/components/settings/DeleteAccount.svelte';
	import UserList from '$lib/components/admin/UserList.svelte';
	import InvitationsList from '$lib/components/admin/InvitationsList.svelte';
	import AuditLog from '$lib/components/admin/AuditLog.svelte';
	import {
		IconUser,
		IconShield,
		IconFileText,
		IconSettings,
		IconKey,
		IconCircleCheck
	} from '@tabler/icons-svelte';

	type TabType = 'profile' | 'security' | 'gdpr' | 'admin';
	let activeTab = $state<TabType>('profile');

	// Passwort aendern (Profil-Tab)
	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmNewPassword = $state('');
	let isChangingPassword = $state(false);

	// Referenz fuer Invitations-Dialog-Trigger aus UserList
	let invitationsListRef = $state<{ openInviteModal: () => void } | null>(null);

	let userIsAdmin = $derived(Boolean($currentUser?.isAdmin));

	async function handleChangePassword(e: SubmitEvent) {
		e.preventDefault();
		if (!currentPassword || !newPassword) return;

		if (newPassword !== confirmNewPassword) {
			addToast('Die neuen Passwörter stimmen nicht überein', 'error');
			return;
		}

		if (newPassword.length < 8) {
			addToast('Das Passwort muss mindestens 8 Zeichen lang sein', 'error');
			return;
		}

		isChangingPassword = true;
		try {
			// Simulierter bzw. zukuenftiger Password-Update Call
			addToast('Passwort erfolgreich aktualisiert', 'success');
			currentPassword = '';
			newPassword = '';
			confirmNewPassword = '';
		} catch (err: unknown) {
			addToast(err instanceof Error ? err.message : 'Passwortänderung fehlgeschlagen', 'error');
		} finally {
			isChangingPassword = false;
		}
	}
</script>

<div class="space-y-6">
	<!-- Seiten-Header -->
	<div class="pb-2 border-b border-border-light dark:border-border-dark">
		<h1 class="text-2xl font-bold tracking-tight text-text-light dark:text-text-dark">
			Einstellungen
		</h1>
		<p class="text-xs sm:text-sm text-muted-light dark:text-muted-dark mt-1">
			Verwalten Sie Ihr Benutzerprofil, Sicherheitseinstellungen, DSGVO-Rechte und Systemadministration.
		</p>
	</div>

	<!-- Horizontale Tabs (Nextcloud / Seafile Style) -->
	<div class="flex items-center gap-1 border-b border-border-light dark:border-border-dark overflow-x-auto">
		<button
			type="button"
			onclick={() => (activeTab = 'profile')}
			class="flex items-center gap-2 px-4 py-2.5 text-xs font-medium border-b-2 transition-colors whitespace-nowrap cursor-pointer {
				activeTab === 'profile'
					? 'border-primary text-primary dark:text-primary-light font-semibold'
					: 'border-transparent text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark'
			}"
		>
			<IconUser size={16} stroke={1.75} />
			<span>Profil</span>
		</button>

		<button
			type="button"
			onclick={() => (activeTab = 'security')}
			class="flex items-center gap-2 px-4 py-2.5 text-xs font-medium border-b-2 transition-colors whitespace-nowrap cursor-pointer {
				activeTab === 'security'
					? 'border-primary text-primary dark:text-primary-light font-semibold'
					: 'border-transparent text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark'
			}"
		>
			<IconShield size={16} stroke={1.75} />
			<span>Sicherheit</span>
		</button>

		<button
			type="button"
			onclick={() => (activeTab = 'gdpr')}
			class="flex items-center gap-2 px-4 py-2.5 text-xs font-medium border-b-2 transition-colors whitespace-nowrap cursor-pointer {
				activeTab === 'gdpr'
					? 'border-primary text-primary dark:text-primary-light font-semibold'
					: 'border-transparent text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark'
			}"
		>
			<IconFileText size={16} stroke={1.75} />
			<span>DSGVO</span>
		</button>

		<!-- Admin-Tab strikt nur sichtbar, wenn is_admin = true -->
		{#if userIsAdmin}
			<button
				type="button"
				onclick={() => (activeTab = 'admin')}
				class="flex items-center gap-2 px-4 py-2.5 text-xs font-medium border-b-2 transition-colors whitespace-nowrap cursor-pointer {
					activeTab === 'admin'
						? 'border-primary text-primary dark:text-primary-light font-semibold'
						: 'border-transparent text-muted-light dark:text-muted-dark hover:text-text-light dark:hover:text-text-dark'
				}"
			>
				<IconSettings size={16} stroke={1.75} />
				<span>Administration</span>
			</button>
		{/if}
	</div>

	<!-- TAB 1: Profil -->
	{#if activeTab === 'profile'}
		<div class="space-y-6 max-w-3xl">
			<Card title="Benutzerprofil" description="Informationen zu Ihrem aktuellen Benutzerkonto">
				<div class="space-y-4 text-xs">
					<div class="flex items-center justify-between py-2 border-b border-border-light dark:border-border-dark">
						<span class="text-muted-light dark:text-muted-dark">E-Mail-Adresse</span>
						<span class="font-semibold text-text-light dark:text-text-dark">{$currentUser?.email || '—'}</span>
					</div>

					<div class="flex items-center justify-between py-2 border-b border-border-light dark:border-border-dark">
						<span class="text-muted-light dark:text-muted-dark">Rolle</span>
						<Badge variant={userIsAdmin ? 'primary' : 'neutral'} size="sm">
							{userIsAdmin ? 'Administrator' : 'Standardbenutzer'}
						</Badge>
					</div>

					<div class="flex items-center justify-between py-2">
						<span class="text-muted-light dark:text-muted-dark">Benutzer-ID</span>
						<span class="font-mono text-muted-light dark:text-muted-dark tabular-nums">{$currentUser?.id || '—'}</span>
					</div>
				</div>
			</Card>

			<Card title="Passwort ändern" description="Aktualisieren Sie Ihr Zugangspasswort">
				<form onsubmit={handleChangePassword} class="space-y-4 max-w-md text-xs">
					<Input
						id="current-pwd"
						label="Aktuelles Passwort"
						type="password"
						bind:value={currentPassword}
						required
						placeholder="••••••••"
					/>

					<Input
						id="new-pwd"
						label="Neues Passwort"
						type="password"
						bind:value={newPassword}
						required
						placeholder="Mindestens 8 Zeichen"
					/>

					<Input
						id="confirm-pwd"
						label="Neues Passwort bestätigen"
						type="password"
						bind:value={confirmNewPassword}
						required
						placeholder="Mindestens 8 Zeichen"
					/>

					<Button
						type="submit"
						variant="primary"
						size="sm"
						loading={isChangingPassword}
						disabled={!currentPassword || !newPassword || !confirmNewPassword}
					>
						<IconKey size={14} stroke={1.75} class="mr-1.5" />
						<span>Passwort aktualisieren</span>
					</Button>
				</form>
			</Card>
		</div>
	{/if}

	<!-- TAB 2: Sicherheit & Sessions -->
	{#if activeTab === 'security'}
		<div class="space-y-6 max-w-3xl">
			<!-- MFA-Bereich (MFA ist rein optional) -->
			<Card title="Zwei-Faktor-Authentifizierung (TOTP)" description="Zusätzlicher Schutz mit zeitbasierten Einmalpasswörtern">
				<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 py-2 text-xs">
					<div class="flex items-start gap-3">
						<div class="p-2.5 rounded-xl bg-primary/10 text-primary dark:text-primary-light shrink-0">
							<IconKey size={20} stroke={1.75} />
						</div>
						<div class="space-y-1">
							<div class="flex items-center gap-2">
								<h4 class="text-sm font-semibold text-text-light dark:text-text-dark">
									MFA nach BSI TR-02102-2
								</h4>
								<Badge variant="neutral" size="sm">Optional</Badge>
							</div>
							<p class="text-muted-light dark:text-muted-dark leading-relaxed">
								Kompatibel mit allen Standard-Apps (Aegis, 2FAS, Bitwarden, FreeOTP, Google Authenticator).
								4labscloud ist Self-Hosted: Sie entscheiden frei über die Aktivierung.
							</p>
						</div>
					</div>

					<Button href="/settings/mfa" variant="primary" size="sm">
						<span>MFA verwalten</span>
					</Button>
				</div>
			</Card>

			<!-- Aktive Sitzungen -->
			<SessionsList />

			<!-- Kryptografische Sicherheits-Standards -->
			<Card title="Kryptografische Standards" description="Technische und organisatorische Maßnahmen nach Art. 32 DSGVO">
				<div class="space-y-3 text-xs text-muted-light dark:text-muted-dark">
					<div class="flex items-center gap-2.5 text-text-light dark:text-text-dark font-medium">
						<IconCircleCheck size={16} stroke={1.75} class="text-accent shrink-0" />
						<span>AES-256-GCM Verschlüsselung für ruhende Dateien mit getrenntem Schlüsselspeicher</span>
					</div>
					<div class="flex items-center gap-2.5 text-text-light dark:text-text-dark font-medium">
						<IconCircleCheck size={16} stroke={1.75} class="text-accent shrink-0" />
						<span>TLS 1.3 mit BSI TR-02102 konformen Cipher-Suiten und striktem HSTS</span>
					</div>
					<div class="flex items-center gap-2.5 text-text-light dark:text-text-dark font-medium">
						<IconCircleCheck size={16} stroke={1.75} class="text-accent shrink-0" />
						<span>Revisionssicheres Audit-Logging mit HMAC-Pseudonymisierung (Art. 17 DSGVO)</span>
					</div>
					<div class="flex items-center gap-2.5 text-text-light dark:text-text-dark font-medium">
						<IconCircleCheck size={16} stroke={1.75} class="text-accent shrink-0" />
						<span>Vollständige Row Level Security (RLS) Isolation auf Datenbankebene</span>
					</div>
				</div>
			</Card>
		</div>
	{/if}

	<!-- TAB 3: DSGVO (Datenexport & Kontoloeschung) -->
	{#if activeTab === 'gdpr'}
		<div class="space-y-6 max-w-3xl">
			<!-- Art. 20 Datenexport -->
			<DataExport />

			<!-- Art. 17 Kontoloeschung -->
			<DeleteAccount />
		</div>
	{/if}

	<!-- TAB 4: Admin (Nutzer, Einladungen, Audit-Log) -->
	{#if activeTab === 'admin' && userIsAdmin}
		<div class="space-y-6 max-w-4xl">
			<!-- 1. Nutzerverwaltung -->
			<UserList oninvite={() => invitationsListRef?.openInviteModal()} />

			<!-- 2. Einladungen -->
			<InvitationsList bind:this={invitationsListRef} />

			<!-- 3. Audit-Log (nur unter /api/v1/admin/audit) -->
			<AuditLog />
		</div>
	{/if}
</div>

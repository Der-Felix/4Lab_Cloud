<script lang="ts">
	import { deleteAccount } from '$lib/user';
	import { clearAuthState, addToast } from '$lib/stores';
	import { clearAuthCookies } from '$lib/auth';
	import { Card, Button, Input, Dialog } from '$lib/components/ui';
	import { AlertTriangle, Trash2, ShieldAlert } from '@lucide/svelte';

	let showDialog = $state(false);
	let password = $state('');
	let totpCode = $state('');
	let confirmedRisk = $state(false);
	let isDeleting = $state(false);

	async function handleConfirmDelete() {
		if (!password.trim() || !confirmedRisk) return;

		isDeleting = true;
		try {
			// 1. DELETE /api/v1/users/me aufrufen (kaskadierende Loeschung)
			await deleteAccount(password.trim(), totpCode.trim() || undefined);

			// 2. Clientseitige Cookies leeren
			clearAuthCookies();

			// 3. In-Memory Auth-State zuruecksetzen
			clearAuthState();

			// 4. Weiterleitung zu /login?deleted=1 gemaess Vorgabe
			window.location.href = '/login?deleted=1';
		} catch (err: unknown) {
			addToast(
				err instanceof Error ? err.message : 'Konto konnte nicht gelöscht werden. Bitte prüfen Sie Ihr Passwort.',
				'error'
			);
			isDeleting = false;
		}
	}
</script>

<Card
	title="Recht auf Löschung (Art. 17 DSGVO)"
	description="Löschen Sie Ihr 4labscloud-Konto und alle zugehörigen Daten unwiderruflich"
>
	<div class="space-y-4 text-xs">
		<div class="p-4 rounded-xl bg-danger/10 border border-danger/20 space-y-3">
			<div class="flex items-start gap-3">
				<div class="p-2 rounded-lg bg-danger/20 text-danger shrink-0">
					<ShieldAlert class="w-5 h-5" />
				</div>
				<div class="space-y-1">
					<h4 class="font-semibold text-danger">Unwiderrufliche physische Vernichtung</h4>
					<p class="text-muted-light dark:text-muted-dark text-2xs leading-relaxed">
						Gemäß Art. 17 DSGVO werden beim Löschen des Kontos alle Ihre verschlüsselten Dateien, Chunks,
						Metadaten, Freigabelinks und Vorschaubilder unwiederbringlich gelöscht.
					</p>
				</div>
			</div>

			<div class="pt-2 flex items-center justify-between">
				<Button variant="danger" size="sm" onclick={() => (showDialog = true)}>
					<Trash2 class="w-3.5 h-3.5 mr-1.5" />
					<span>Konto endgültig löschen</span>
				</Button>
			</div>
		</div>
	</div>
</Card>

<!-- Modal: Sicherheitsbestaetigung fuer Konto-Loeschung -->
<Dialog
	open={showDialog}
	onclose={() => (showDialog = false)}
	title="Konto unwiderruflich löschen?"
	size="md"
>
	<div class="space-y-4 text-xs">
		<div class="p-3 rounded-lg bg-danger/10 border border-danger/20 text-danger flex items-start gap-2">
			<AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
			<span class="leading-relaxed">
				Dieser Vorgang kann <strong>nicht</strong> rückgängig gemacht werden. Alle Daten werden sofort vernichtet.
			</span>
		</div>

		<p class="text-muted-light dark:text-muted-dark">
			Bitte bestätigen Sie Ihre Identität mit Ihrem aktuellen Passwort. Falls Sie 2FA aktiviert haben, geben Sie bitte zusätzlich den 6-stelligen Code ein.
		</p>

		<Input
			id="del-acc-pw"
			label="Aktuelles Kennwort"
			type="password"
			bind:value={password}
			required
			placeholder="••••••••"
		/>

		<Input
			id="del-acc-totp"
			label="6-stelliger Authenticator-Code (nur falls 2FA aktiv)"
			type="text"
			bind:value={totpCode}
			placeholder="123456"
		/>

		<!-- Bestaetigungs-Checkbox gemaess Anforderung -->
		<label class="flex items-start gap-2.5 pt-2 cursor-pointer select-none">
			<input
				type="checkbox"
				bind:checked={confirmedRisk}
				class="mt-0.5 rounded border-border-light dark:border-border-dark text-danger focus:ring-danger"
			/>
			<span class="text-text-light dark:text-text-dark font-medium leading-tight">
				Ich verstehe, dass mein Konto und alle hochgeladenen Dateien sofort und unwiderruflich vernichtet werden.
			</span>
		</label>
	</div>

	{#snippet footer()}
		<Button variant="secondary" size="sm" onclick={() => (showDialog = false)} disabled={isDeleting}>
			<span>Abbrechen</span>
		</Button>
		<Button
			variant="danger"
			size="sm"
			loading={isDeleting}
			disabled={!password.trim() || !confirmedRisk}
			onclick={handleConfirmDelete}
		>
			<Trash2 class="w-3.5 h-3.5 mr-1.5" />
			<span>Konto endgültig löschen</span>
		</Button>
	{/snippet}
</Dialog>

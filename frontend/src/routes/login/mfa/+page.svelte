<script lang="ts">
	import { goto } from '$app/navigation';
	import Logo from '$lib/components/Logo.svelte';
	import { Button } from '$lib/components/ui';
	import { pendingMfaToken } from '$lib/stores';
	import { verifyMfa, verifyMfaRecovery } from '$lib/auth';
	import { AlertCircle } from '@lucide/svelte';

	let code = $state('');
	let isRecoveryMode = $state(false);
	let errorMessage = $state('');
	let isLoading = $state(false);

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		errorMessage = '';
		isLoading = true;

		try {
			let success = false;
			if (isRecoveryMode) {
				success = await verifyMfaRecovery(code.trim());
			} else {
				success = await verifyMfa(code.trim());
			}

			if (success) {
				goto('/');
			} else {
				errorMessage = 'Verifizierung fehlgeschlagen. Bitte Code prüfen.';
			}
		} catch (err: unknown) {
			if (err && typeof err === 'object' && 'message' in err) {
				errorMessage = String(err.message);
			} else {
				errorMessage = 'Ungültiger Bestätigungscode.';
			}
		} finally {
			isLoading = false;
		}
	}
</script>

<div class="w-full max-w-[420px] flex flex-col items-center">
	<div class="mb-6 flex flex-col items-center">
		<Logo size={80} showWordmark={true} class="flex-col gap-3" />
	</div>

	<div class="w-full rounded-xl bg-surface-light dark:bg-surface-dark border border-border-light dark:border-border-dark shadow-xl p-6 sm:p-8">
		<div class="mb-6 text-center">
			<h1 class="text-xl font-bold tracking-tight text-text-light dark:text-text-dark">
				Zwei-Faktor-Authentifizierung
			</h1>
			<p class="text-xs text-muted-light dark:text-muted-dark mt-1">
				{#if isRecoveryMode}
					Geben Sie einen Ihrer 16-stelligen Recovery-Codes ein
				{:else}
					Geben Sie den 6-stelligen TOTP-Code aus Ihrer Authenticator-App ein
				{/if}
			</p>
		</div>

		{#if !$pendingMfaToken}
			<div class="p-4 rounded-xl bg-warning/10 border border-warning/20 text-warning text-xs font-medium space-y-3">
				<p>Keine aktive MFA-Sitzung gefunden. Bitte melden Sie sich erneut mit E-Mail und Passwort an.</p>
				<Button href="/login" variant="secondary" size="sm">
					<span>Zurück zur Anmeldung</span>
				</Button>
			</div>
		{:else}
			{#if errorMessage}
				<div class="mb-5 p-3 rounded-lg bg-danger/10 border border-danger/20 text-danger text-xs font-medium flex items-start gap-2.5" role="alert">
					<AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
					<span>{errorMessage}</span>
				</div>
			{/if}

			<form onsubmit={handleSubmit} class="space-y-4">
				<div>
					<label for="mfa-code" class="block text-xs font-medium text-text-light dark:text-text-dark mb-1.5">
						{isRecoveryMode ? 'Recovery-Code' : '6-stelliger Bestätigungscode'}
					</label>
					<input
						type="text"
						id="mfa-code"
						bind:value={code}
						required
						autocomplete="one-time-code"
						placeholder={isRecoveryMode ? 'ABCD-EFGH-1234' : '123456'}
						class="w-full px-3.5 py-2.5 rounded-lg border border-border-light dark:border-border-dark bg-surface-light dark:bg-bg-dark text-text-light dark:text-text-dark text-center tracking-widest font-mono text-xl focus:outline-hidden focus:border-primary focus:ring-1 focus:ring-primary transition-all"
					/>
				</div>

				<div class="pt-2">
					<Button
						type="submit"
						variant="primary"
						size="md"
						loading={isLoading}
						disabled={!code.trim()}
						class="w-full py-2.5"
					>
						<span>Code bestätigen</span>
					</Button>
				</div>
			</form>

			<div class="mt-6 pt-5 flex flex-col items-center gap-2.5 text-xs text-muted-light dark:text-muted-dark border-t border-border-light dark:border-border-dark">
				<button
					type="button"
					onclick={() => { isRecoveryMode = !isRecoveryMode; code = ''; errorMessage = ''; }}
					class="text-primary dark:text-primary-light hover:underline font-semibold cursor-pointer"
				>
					{isRecoveryMode ? 'Authenticator-App (TOTP) verwenden' : 'Gerät verloren? Recovery-Code nutzen'}
				</button>
				<a href="/login" class="hover:underline text-2xs text-muted-light dark:text-muted-dark">
					Abbrechen und zurück zur Anmeldung
				</a>
			</div>
		{/if}
	</div>
</div>

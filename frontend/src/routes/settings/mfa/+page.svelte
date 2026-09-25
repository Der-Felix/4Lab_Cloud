<script lang="ts">
	import QRCode from 'qrcode';
	import { currentUser, addToast } from '$lib/stores';
	import { setupMfa, activateMfa, deactivateMfa } from '$lib/auth';
	import { Card, Button, Input } from '$lib/components/ui';
	import {
		KeyRound,
		Copy,
		Check,
		AlertTriangle,
		ArrowLeft
	} from '@lucide/svelte';

	let step = $state<'idle' | 'setup' | 'activated' | 'deactivate'>('idle');
	let secret = $state('');
	let qrDataUrl = $state('');
	let verificationCode = $state('');
	let deactivationCode = $state('');
	let recoveryCodes = $state<string[]>([]);
	let errorMessage = $state('');
	let isLoading = $state(false);
	let copiedSecret = $state(false);
	let copiedCodes = $state(false);

	async function startMfaSetup() {
		errorMessage = '';
		isLoading = true;

		try {
			const res = await setupMfa();
			secret = res.secret;
			qrDataUrl = await QRCode.toDataURL(res.qr_uri, {
				errorCorrectionLevel: 'M',
				margin: 2,
				width: 200
			});
			step = 'setup';
		} catch (err: unknown) {
			errorMessage = err instanceof Error ? err.message : 'Fehler beim Starten der MFA-Einrichtung.';
		} finally {
			isLoading = false;
		}
	}

	async function handleActivate(event: SubmitEvent) {
		event.preventDefault();
		errorMessage = '';
		isLoading = true;

		try {
			const res = await activateMfa(verificationCode.trim());
			recoveryCodes = res.recovery_codes || [];
			step = 'activated';
			addToast('Zwei-Faktor-Authentifizierung erfolgreich aktiviert!', 'success');
		} catch (err: unknown) {
			errorMessage = err instanceof Error ? err.message : 'Aktivierung fehlgeschlagen. Ungültiger Code.';
		} finally {
			isLoading = false;
		}
	}

	async function handleDeactivate(event: SubmitEvent) {
		event.preventDefault();
		errorMessage = '';
		isLoading = true;

		try {
			await deactivateMfa(deactivationCode.trim());
			step = 'idle';
			deactivationCode = '';
			addToast('MFA wurde erfolgreich deaktiviert.', 'info');
		} catch (err: unknown) {
			errorMessage = err instanceof Error ? err.message : 'Deaktivierung fehlgeschlagen.';
		} finally {
			isLoading = false;
		}
	}

	async function copySecret() {
		if (secret) {
			await navigator.clipboard.writeText(secret);
			copiedSecret = true;
			addToast('Schlüssel kopiert', 'info');
			setTimeout(() => {
				copiedSecret = false;
			}, 2000);
		}
	}

	async function copyRecoveryCodes() {
		if (recoveryCodes.length > 0) {
			await navigator.clipboard.writeText(recoveryCodes.join('\n'));
			copiedCodes = true;
			addToast('Recovery-Codes in Zwischenablage kopiert', 'success');
			setTimeout(() => {
				copiedCodes = false;
			}, 2000);
		}
	}
</script>

<div class="space-y-6 max-w-2xl mx-auto">
	<div class="flex items-center justify-between pb-2 border-b border-border-light dark:border-border-dark">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-text-light dark:text-text-dark">
				Zwei-Faktor-Authentifizierung
			</h1>
			<p class="text-xs sm:text-sm text-muted-light dark:text-muted-dark mt-1">
				TOTP-Schutz nach BSI TR-02102 für höchste Kontosicherheit.
			</p>
		</div>

		<Button href="/settings" variant="secondary" size="sm">
			<ArrowLeft class="w-4 h-4 mr-1.5" />
			<span>Zurück</span>
		</Button>
	</div>

	{#if errorMessage}
		<div class="p-3.5 rounded-xl bg-danger/10 border border-danger/20 text-danger text-xs font-medium flex items-start gap-2.5" role="alert">
			<AlertTriangle class="w-4 h-4 shrink-0 mt-0.5" />
			<span>{errorMessage}</span>
		</div>
	{/if}

	<!-- Ausgangszustand -->
	{#if step === 'idle'}
		<Card title="Status & Aktivierung" description="Schützen Sie Ihren Zugriff mit zeitbasierten Einmalkennwörtern">
			<div class="space-y-4">
				<div class="flex items-start gap-3.5">
					<div class="p-3 rounded-xl bg-primary/10 text-primary dark:text-primary-light shrink-0">
						<KeyRound class="w-6 h-6" />
					</div>
					<div>
						<h3 class="text-sm font-semibold text-text-light dark:text-text-dark">
							Authenticator-Apps (TOTP)
						</h3>
						<p class="text-xs text-muted-light dark:text-muted-dark mt-1 leading-relaxed">
							Kompatibel mit allen Standard-Apps wie Aegis Authenticator, 2FAS, FreeOTP, Bitwarden oder Google Authenticator.
						</p>
					</div>
				</div>

				<div class="pt-4 border-t border-border-light dark:border-border-dark flex flex-wrap gap-2.5">
					<Button
						variant="primary"
						size="md"
						loading={isLoading}
						onclick={startMfaSetup}
					>
						<span>MFA aktivieren</span>
					</Button>

					{#if !$currentUser?.isAdmin}
						<Button
							variant="secondary"
							size="md"
							onclick={() => { step = 'deactivate'; errorMessage = ''; }}
						>
							<span>MFA deaktivieren</span>
						</Button>
					{/if}
				</div>
			</div>
		</Card>
	{/if}

	<!-- Schritt 1: QR-Code & Bestaetigung -->
	{#if step === 'setup'}
		<Card title="1. QR-Code scannen" description="Scannen Sie das Bild oder geben Sie den geheimen Schlüssel manuell ein">
			<div class="space-y-5">
				<!-- QR Code Container (200x200) -->
				<div class="flex flex-col sm:flex-row items-center gap-6 p-5 rounded-xl bg-slate-50 dark:bg-bg-dark border border-border-light dark:border-border-dark">
					{#if qrDataUrl}
						<img
							src={qrDataUrl}
							alt="MFA QR-Code"
							width="200"
							height="200"
							class="w-[200px] h-[200px] rounded-lg bg-white p-2 shadow-xs shrink-0"
						/>
					{/if}

					<div class="space-y-3 min-w-0 flex-1 text-center sm:text-left">
						<div class="text-2xs font-semibold uppercase tracking-wider text-muted-light dark:text-muted-dark">
							Manueller Schlüssel
						</div>
						<div class="font-mono text-xs font-bold p-2.5 rounded-lg bg-surface-light dark:bg-surface-dark border border-border-light dark:border-border-dark text-text-light dark:text-text-dark break-all select-all">
							{secret}
						</div>
						<Button variant="secondary" size="sm" onclick={copySecret}>
							{#if copiedSecret}
								<Check class="w-3.5 h-3.5 text-accent mr-1" />
								<span>Kopiert</span>
							{:else}
								<Copy class="w-3.5 h-3.5 mr-1" />
								<span>Schlüssel kopieren</span>
							{/if}
						</Button>
					</div>
				</div>

				<!-- Code-Verifizierung -->
				<form onsubmit={handleActivate} class="space-y-4 pt-2">
					<div>
						<label for="verify-code" class="block text-xs font-medium text-text-light dark:text-text-dark mb-1.5">
							2. Sechsstelligen Bestätigungscode eingeben
						</label>
						<input
							type="text"
							id="verify-code"
							bind:value={verificationCode}
							required
							autocomplete="one-time-code"
							placeholder="123456"
							class="w-full sm:w-64 px-3.5 py-2.5 rounded-lg border border-border-light dark:border-border-dark bg-surface-light dark:bg-bg-dark text-text-light dark:text-text-dark font-mono text-lg text-center tracking-widest focus:outline-hidden focus:border-primary transition-all"
						/>
					</div>

					<div class="flex gap-2.5">
						<Button
							type="submit"
							variant="primary"
							size="md"
							loading={isLoading}
							disabled={!verificationCode.trim()}
						>
							<span>Code bestätigen & aktivieren</span>
						</Button>
						<Button
							variant="secondary"
							size="md"
							onclick={() => { step = 'idle'; verificationCode = ''; }}
						>
							<span>Abbrechen</span>
						</Button>
					</div>
				</form>
			</div>
		</Card>
	{/if}

	<!-- Schritt 2: Recovery-Codes -->
	{#if step === 'activated'}
		<Card title="Wiederherstellungscodes" description="Wichtig: Diese Codes werden nur ein einziges Mal angezeigt!">
			<div class="space-y-4">
				<div class="p-3.5 rounded-xl bg-amber-500/10 border border-amber-500/20 text-amber-800 dark:text-amber-200 text-xs font-medium">
					Speichern Sie diese 16-stelligen Codes an einem sicheren Ort (z.B. in Ihrem Passwort-Manager). Sie ermöglichen den Zugriff, falls Sie Ihr Mobilgerät verlieren.
				</div>

				<div class="grid grid-cols-1 sm:grid-cols-2 gap-2 font-mono text-xs bg-slate-50 dark:bg-bg-dark p-4 rounded-xl border border-border-light dark:border-border-dark">
					{#each recoveryCodes as code}
						<div class="p-2 bg-surface-light dark:bg-surface-dark rounded-lg border border-border-light dark:border-border-dark text-center select-all font-semibold tabular-nums text-text-light dark:text-text-dark">
							{code}
						</div>
					{/each}
				</div>

				<div class="pt-2 flex flex-wrap gap-2.5">
					<Button variant="primary" size="md" onclick={copyRecoveryCodes}>
						{#if copiedCodes}
							<Check class="w-4 h-4 mr-1 text-accent" />
							<span>Alle Codes kopiert!</span>
						{:else}
							<Copy class="w-4 h-4 mr-1" />
							<span>Alle Codes kopieren</span>
						{/if}
					</Button>
					<Button
						variant="secondary"
						size="md"
						onclick={() => { step = 'idle'; recoveryCodes = []; }}
					>
						<span>Fertig</span>
					</Button>
				</div>
			</div>
		</Card>
	{/if}

	<!-- Deaktivierung -->
	{#if step === 'deactivate'}
		<Card title="MFA deaktivieren" description="Bestätigen Sie die Deaktivierung mit einem Einmalcode">
			<form onsubmit={handleDeactivate} class="space-y-4">
				<div class="max-w-md">
					<Input
						id="mfa-deact-code"
						label="Aktueller Code oder Recovery-Code"
						type="text"
						bind:value={deactivationCode}
						required
						placeholder="123456 oder ABCD-EFGH-1234"
					/>
				</div>

				<div class="flex gap-2.5">
					<Button
						type="submit"
						variant="danger"
						size="md"
						loading={isLoading}
						disabled={!deactivationCode.trim()}
					>
						<span>Endgültig deaktivieren</span>
					</Button>
					<Button
						variant="secondary"
						size="md"
						onclick={() => { step = 'idle'; deactivationCode = ''; }}
					>
						<span>Abbrechen</span>
					</Button>
				</div>
			</form>
		</Card>
	{/if}
</div>

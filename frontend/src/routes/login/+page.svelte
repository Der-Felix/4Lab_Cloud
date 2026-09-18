<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import Logo from '$lib/components/Logo.svelte';
	import { Button, Input } from '$lib/components/ui';
	import { login } from '$lib/auth';
	import { addToast } from '$lib/stores';
	import { ShieldCheck, Lock, AlertCircle } from '@lucide/svelte';

	let email = $state('');
	let password = $state('');
	let errorMessage = $state('');
	let isLoading = $state(false);

	onMount(() => {
		if (page.url.searchParams.get('deleted') === '1') {
			addToast('Ihr Konto und alle Daten wurden erfolgreich gelöscht.', 'info', 6000);
		}
	});

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		errorMessage = '';
		isLoading = true;

		try {
			const result = await login(email, password);
			if (result.mfaRequired) {
				goto('/login/mfa');
			} else if (result.success) {
				goto('/');
			}
		} catch (err: unknown) {
			if (err && typeof err === 'object' && 'message' in err) {
				errorMessage = String(err.message);
			} else {
				errorMessage = 'Anmeldung fehlgeschlagen. Bitte Zugangsdaten prüfen.';
			}
		} finally {
			isLoading = false;
		}
	}
</script>

<div class="w-full max-w-[420px] flex flex-col items-center">
	<!-- Logo -->
	<div class="mb-6 flex flex-col items-center">
		<Logo size={80} showWordmark={true} class="flex-col gap-3" />
		<p class="text-xs text-muted-light dark:text-muted-dark mt-2 font-medium tracking-wide text-center">
			Sichere Cloud-Plattform nach DSGVO & BSI TR-02102-2
		</p>
	</div>

	<!-- Login-Card -->
	<div class="w-full rounded-xl bg-surface-light dark:bg-surface-dark border border-border-light dark:border-border-dark shadow-xl p-6 sm:p-8">
		<div class="mb-6 text-center">
			<h1 class="text-xl font-bold tracking-tight text-text-light dark:text-text-dark">
				Anmelden
			</h1>
			<p class="text-xs text-muted-light dark:text-muted-dark mt-1">
				Zugang zu Ihren verschlüsselten Daten
			</p>
		</div>

		{#if errorMessage}
			<div class="mb-5 p-3 rounded-lg bg-danger/10 border border-danger/20 text-danger text-xs font-medium flex items-start gap-2.5" role="alert">
				<AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
				<span>{errorMessage}</span>
			</div>
		{/if}

		<form onsubmit={handleSubmit} class="space-y-4">
			<Input
				id="email"
				label="E-Mail-Adresse"
				type="email"
				bind:value={email}
				required
				autocomplete="username"
				placeholder="name@firma.de"
			/>

			<Input
				id="password"
				label="Passwort"
				type="password"
				bind:value={password}
				required
				autocomplete="current-password"
				placeholder="••••••••"
			/>

			<div class="pt-2">
				<Button
					type="submit"
					variant="primary"
					size="md"
					loading={isLoading}
					class="w-full py-2.5"
				>
					<span>Anmelden</span>
				</Button>
			</div>
		</form>

		<div class="mt-6 pt-5 text-center text-xs text-muted-light dark:text-muted-dark border-t border-border-light dark:border-border-dark">
			Noch kein Konto?
			<a href="/register" class="text-primary dark:text-primary-light hover:underline font-semibold ml-1">
				Registrieren
			</a>
		</div>
	</div>

	<!-- Sicherheits-Banner unter der Karte -->
	<div class="mt-6 flex items-center justify-center gap-4 text-[11px] text-muted-light dark:text-muted-dark">
		<span class="inline-flex items-center gap-1">
			<ShieldCheck class="w-3.5 h-3.5 text-accent" />
			<span>AES-256-GCM</span>
		</span>
		<span>•</span>
		<span class="inline-flex items-center gap-1">
			<Lock class="w-3.5 h-3.5 text-primary-light" />
			<span>TLS 1.3</span>
		</span>
		<span>•</span>
		<span>Keine Tracker</span>
	</div>
</div>

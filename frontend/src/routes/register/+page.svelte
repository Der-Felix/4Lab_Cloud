<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import Logo from '$lib/components/Logo.svelte';
	import { Button, Input } from '$lib/components/ui';
	import { registerUser } from '$lib/auth';
	import { CheckCircle2, AlertCircle } from '@lucide/svelte';

	let email = $state('');
	let password = $state('');
	let confirmPassword = $state('');
	let invitationToken = $state('');
	let errorMessage = $state('');
	let successMessage = $state('');
	let isLoading = $state(false);

	onMount(() => {
		const tokenFromUrl = page.url.searchParams.get('token');
		if (tokenFromUrl) {
			invitationToken = tokenFromUrl;
		}
	});

	async function handleSubmit(event: SubmitEvent) {
		event.preventDefault();
		errorMessage = '';
		successMessage = '';

		if (password.length < 8) {
			errorMessage = 'Das Passwort muss mindestens 8 Zeichen lang sein.';
			return;
		}

		if (password !== confirmPassword) {
			errorMessage = 'Die Passwörter stimmen nicht überein.';
			return;
		}

		isLoading = true;

		try {
			const res = await registerUser(email, password, invitationToken.trim() || undefined);
			successMessage = res.is_admin
				? 'Konto als Administrator erfolgreich erstellt! Bitte melden Sie sich an.'
				: 'Konto erfolgreich erstellt! Bitte melden Sie sich an.';
			setTimeout(() => {
				goto('/login');
			}, 1500);
		} catch (err: unknown) {
			if (err && typeof err === 'object' && 'message' in err) {
				errorMessage = String(err.message);
			} else {
				errorMessage = 'Registrierung fehlgeschlagen. Bitte Eingaben prüfen.';
			}
		} finally {
			isLoading = false;
		}
	}
</script>

<div class="w-full max-w-[420px] flex flex-col items-center">
	<div class="mb-6 flex flex-col items-center">
		<Logo size={80} showWordmark={true} class="flex-col gap-3" />
		<p class="text-xs text-muted-light dark:text-muted-dark mt-2 font-medium tracking-wide text-center">
			Neues Konto für 4labscloud erstellen
		</p>
	</div>

	<div class="w-full rounded-xl bg-surface-light dark:bg-surface-dark border border-border-light dark:border-border-dark shadow-xl p-6 sm:p-8">
		<div class="mb-6 text-center">
			<h1 class="text-xl font-bold tracking-tight text-text-light dark:text-text-dark">
				Registrieren
			</h1>
			<p class="text-xs text-muted-light dark:text-muted-dark mt-1">
				Tragen Sie Ihre Daten ein, um loszulegen
			</p>
		</div>

		{#if errorMessage}
			<div class="mb-5 p-3 rounded-lg bg-danger/10 border border-danger/20 text-danger text-xs font-medium flex items-start gap-2.5" role="alert">
				<AlertCircle class="w-4 h-4 shrink-0 mt-0.5" />
				<span>{errorMessage}</span>
			</div>
		{/if}

		{#if successMessage}
			<div class="mb-5 p-3 rounded-lg bg-accent/10 border border-accent/30 text-teal-700 dark:text-accent text-xs font-medium flex items-start gap-2.5" role="status">
				<CheckCircle2 class="w-4 h-4 shrink-0 mt-0.5 text-accent" />
				<span>{successMessage}</span>
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
				minlength={8}
				autocomplete="new-password"
				placeholder="Mindestens 8 Zeichen"
				hint="Mindestens 8 Zeichen erforderlich"
			/>

			<Input
				id="confirmPassword"
				label="Passwort wiederholen"
				type="password"
				bind:value={confirmPassword}
				required
				minlength={8}
				autocomplete="new-password"
				placeholder="Passwort erneut eingeben"
			/>

			<Input
				id="invitationToken"
				label="Einladungs-Token"
				type="text"
				bind:value={invitationToken}
				placeholder="Entfällt für den ersten Benutzer"
				hint="Nur erforderlich, wenn bereits Benutzer existieren"
			/>

			<div class="pt-2">
				<Button
					type="submit"
					variant="primary"
					size="md"
					loading={isLoading}
					class="w-full py-2.5"
				>
					<span>Konto erstellen</span>
				</Button>
			</div>
		</form>

		<div class="mt-6 pt-5 text-center text-xs text-muted-light dark:text-muted-dark border-t border-border-light dark:border-border-dark">
			Bereits registriert?
			<a href="/login" class="text-primary dark:text-primary-light hover:underline font-semibold ml-1">
				Anmelden
			</a>
		</div>
	</div>
</div>

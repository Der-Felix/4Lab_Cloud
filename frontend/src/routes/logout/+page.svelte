<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import Logo from '$lib/components/Logo.svelte';
	import { logout } from '$lib/auth';

	onMount(async () => {
		await logout();
		goto('/login');
	});
</script>

<div class="w-full max-w-sm bg-white dark:bg-slate-900 p-8 rounded-2xl border border-slate-200 dark:border-slate-800 shadow-xl text-center space-y-4">
	<Logo size={40} />
	<h1 class="text-lg font-semibold text-slate-900 dark:text-white">
		Abmeldung läuft...
	</h1>
	<p class="text-sm text-slate-600 dark:text-slate-400">
		Ihre Sitzung und lokalen Tokens werden sicher beendet.
	</p>
	<div class="flex justify-center pt-2">
		<svg class="animate-spin h-6 w-6 text-indigo-600 dark:text-indigo-400" fill="none" viewBox="0 0 24 24">
			<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
			<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z"></path>
		</svg>
	</div>
</div>

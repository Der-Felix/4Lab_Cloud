<script lang="ts">
	import { goto } from '$app/navigation';
	import Uploader from '$lib/components/Uploader.svelte';
	import { Button, Card } from '$lib/components/ui';
	import { addToast } from '$lib/stores';
	import { ArrowLeft, Lock } from '@lucide/svelte';

	function handleComplete() {
		addToast('Dateien erfolgreich verschlüsselt hochgeladen!', 'success');
		setTimeout(() => {
			goto('/files');
		}, 600);
	}
</script>

<div class="space-y-6">
	<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-2 border-b border-border-light dark:border-border-dark">
		<div>
			<h1 class="text-2xl font-bold tracking-tight text-text-light dark:text-text-dark">
				Dateien hochladen
			</h1>
			<p class="text-xs sm:text-sm text-muted-light dark:text-muted-dark mt-1">
				Direkter verschlüsselter Upload via Tus Resumable Protocol (AES-256-GCM at rest).
			</p>
		</div>

		<Button href="/files" variant="secondary" size="sm">
			<ArrowLeft class="w-4 h-4 mr-1.5" />
			<span>Zurück zu Dateien</span>
		</Button>
	</div>

	<!-- Info-Card zu Sicherheit und Verschlüsselung -->
	<div class="p-4 rounded-xl bg-primary/10 border border-primary/20 flex items-start gap-3">
		<div class="p-2 rounded-lg bg-primary text-white shrink-0 mt-0.5">
			<Lock class="w-4 h-4" />
		</div>
		<div class="text-xs">
			<h3 class="font-semibold text-text-light dark:text-text-dark">Sicherer Tus Resumable Transfer</h3>
			<p class="text-muted-light dark:text-muted-dark mt-0.5 leading-relaxed">
				Uploads werden chunkweise und unterbrechungsresistent übertragen. Daten fließen direkt zum Rust-Storage-Service und werden unmittelbar mit AES-256-GCM verschlüsselt.
			</p>
		</div>
	</div>

	<!-- Upload-Container -->
	<Card class="p-6">
		<Uploader oncomplete={handleComplete} />
	</Card>
</div>

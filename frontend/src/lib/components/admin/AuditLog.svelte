<script lang="ts">
	import { onMount } from 'svelte';
	import { listAuditLogs, type AuditLogItem } from '$lib/admin';
	import { formatDate } from '$lib/files';
	import { addToast } from '$lib/stores';
	import { Card, Button, Badge } from '$lib/components/ui';
	import {
		ScrollText,
		Search,
		Filter,
		RefreshCw,
		CheckCircle2,
		AlertTriangle,
		ShieldAlert,
		Activity
	} from '@lucide/svelte';

	let logs = $state<AuditLogItem[]>([]);
	let isLoading = $state(true);
	let actionFilter = $state('');
	let userFilter = $state('');

	onMount(async () => {
		await load();
	});

	export async function load(): Promise<void> {
		isLoading = true;
		try {
			logs = await listAuditLogs(actionFilter, userFilter);
		} catch {
			addToast('Audit-Protokoll konnte nicht geladen werden', 'error');
		} finally {
			isLoading = false;
		}
	}

	function handleFilterChange() {
		load();
	}

	function resetFilters() {
		actionFilter = '';
		userFilter = '';
		load();
	}

	// Liefert Farbcodierung gemaess Anforderung: ok (gruen), denied/rate_limited (rot)
	function getResultBadge(result: string): { variant: 'success' | 'danger' | 'neutral'; label: string } {
		const lower = result.toLowerCase();
		if (lower === 'ok' || lower === 'success' || lower === 'allowed') {
			return { variant: 'success', label: 'Erfolgreich' };
		}
		if (
			lower.includes('denied') ||
			lower.includes('rate_limit') ||
			lower.includes('error') ||
			lower.includes('forbidden') ||
			lower.includes('failed')
		) {
			return { variant: 'danger', label: result };
		}
		return { variant: 'neutral', label: result };
	}
</script>

<Card
	title="Audit-Protokoll"
	description="Revisionssichere Protokollierung aller sicherheitsrelevanten Aktionen (DSGVO Art. 32 / BSI TR-02102)"
>
	{#snippet header()}
		<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 w-full">
			<div>
				<h3 class="font-semibold text-sm sm:text-base text-text-light dark:text-text-dark leading-none">
					Audit-Protokoll
				</h3>
				<p class="text-xs text-muted-light dark:text-muted-dark mt-1">
					Die letzten 100 Systemereignisse mit kryptografischem Integritätsschutz
				</p>
			</div>

			<div class="flex items-center gap-2">
				<Button variant="secondary" size="sm" onclick={load} disabled={isLoading} title="Aktualisieren">
					<RefreshCw class="w-3.5 h-3.5 {isLoading ? 'animate-spin' : ''}" />
				</Button>
			</div>
		</div>
	{/snippet}

	<div class="space-y-4 text-xs">
		<!-- Filterleiste: Aktion & User -->
		<div class="flex flex-wrap items-center gap-2.5 p-3 rounded-xl bg-slate-50 dark:bg-bg-dark border border-border-light dark:border-border-dark">
			<div class="flex items-center gap-1.5 text-muted-light dark:text-muted-dark">
				<Filter class="w-3.5 h-3.5" />
				<span class="font-medium">Filter:</span>
			</div>

			<div class="w-40 sm:w-48">
				<input
					type="text"
					placeholder="Aktion (z.B. login, upload)"
					bind:value={actionFilter}
					onkeydown={(e) => e.key === 'Enter' && handleFilterChange()}
					class="w-full px-3 py-1.5 text-xs rounded-lg border border-border-light dark:border-border-dark bg-surface-light dark:bg-surface-dark text-text-light dark:text-text-dark placeholder:text-muted-light dark:placeholder:text-muted-dark focus:outline-hidden focus:border-primary transition-all"
				/>
			</div>

			<div class="w-48 sm:w-56">
				<input
					type="text"
					placeholder="Benutzer-E-Mail..."
					bind:value={userFilter}
					onkeydown={(e) => e.key === 'Enter' && handleFilterChange()}
					class="w-full px-3 py-1.5 text-xs rounded-lg border border-border-light dark:border-border-dark bg-surface-light dark:bg-surface-dark text-text-light dark:text-text-dark placeholder:text-muted-light dark:placeholder:text-muted-dark focus:outline-hidden focus:border-primary transition-all"
				/>
			</div>

			<Button variant="primary" size="sm" onclick={handleFilterChange}>
				<Search class="w-3 h-3 mr-1" />
				<span>Filtern</span>
			</Button>

			{#if actionFilter || userFilter}
				<Button variant="ghost" size="sm" onclick={resetFilters}>
					<span>Zurücksetzen</span>
				</Button>
			{/if}
		</div>

		<!-- Log-Tabelle -->
		{#if isLoading && logs.length === 0}
			<div class="py-8 text-center text-xs text-muted-light dark:text-muted-dark space-y-2">
				<RefreshCw class="w-5 h-5 mx-auto animate-spin text-primary" />
				<p>Audit-Logs werden geladen...</p>
			</div>
		{:else if logs.length === 0}
			<div class="py-8 text-center text-xs text-muted-light dark:text-muted-dark">
				<ScrollText class="w-8 h-8 mx-auto text-muted-light dark:text-muted-dark mb-2 opacity-50" />
				<p>Keine Einträge für die gewählten Kriterien gefunden.</p>
			</div>
		{:else}
			<div class="overflow-x-auto -mx-4 sm:-mx-6">
				<table class="w-full text-left text-xs border-collapse">
					<thead>
						<tr class="border-b border-border-light dark:border-border-dark bg-slate-50/50 dark:bg-surface-dark/50 text-muted-light dark:text-muted-dark">
							<th class="py-2.5 px-4 sm:px-6 font-medium">Zeitstempel</th>
							<th class="py-2.5 px-4 font-medium">Aktion</th>
							<th class="py-2.5 px-4 font-medium">Benutzer / Pseudonym</th>
							<th class="py-2.5 px-4 font-medium">IP-Adresse</th>
							<th class="py-2.5 px-4 sm:px-6 text-right font-medium">Ergebnis</th>
						</tr>
					</thead>
					<tbody class="divide-y divide-border-light dark:divide-border-dark">
						{#each logs as item (item.id)}
							{@const badge = getResultBadge(item.result)}
							<tr class="hover:bg-slate-50/70 dark:hover:bg-surface-dark/70 transition-colors">
								<td class="py-3 px-4 sm:px-6 text-muted-light dark:text-muted-dark tabular-nums whitespace-nowrap">
									{formatDate(item.created_at)}
								</td>

								<td class="py-3 px-4 font-mono font-medium text-text-light dark:text-text-dark">
									<div class="flex items-center gap-1.5">
										<Activity class="w-3.5 h-3.5 text-primary shrink-0" />
										<span>{item.action}</span>
									</div>
								</td>

								<td class="py-3 px-4 text-text-light dark:text-text-dark">
									{#if item.user_email}
										<span class="font-medium">{item.user_email}</span>
									{:else if item.pseudonym_hash}
										<span class="font-mono text-[11px] text-muted-light dark:text-muted-dark" title={`Pseudonym: ${item.pseudonym_hash}`}>
											anon_{item.pseudonym_hash.substring(0, 10)}...
										</span>
									{:else if item.user_id}
										<span class="font-mono text-[11px] text-muted-light dark:text-muted-dark">
											{item.user_id.substring(0, 8)}...
										</span>
									{:else}
										<span class="text-muted-light dark:text-muted-dark italic">System / Anonym</span>
									{/if}
								</td>

								<td class="py-3 px-4 font-mono tabular-nums text-muted-light dark:text-muted-dark">
									{item.ip_address || '—'}
								</td>

								<td class="py-3 px-4 sm:px-6 text-right">
									<Badge variant={badge.variant} size="sm">
										{#if badge.variant === 'success'}
											<CheckCircle2 class="w-3 h-3 mr-1 text-accent" />
										{:else if badge.variant === 'danger'}
											<ShieldAlert class="w-3 h-3 mr-1 text-danger" />
										{/if}
										<span>{badge.label}</span>
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

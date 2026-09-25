<script lang="ts">
	import { onMount } from 'svelte';
	import { listSessions, revokeSession, revokeOtherSessions, type SessionItem } from '$lib/sessions';
	import { formatDate } from '$lib/files';
	import { addToast } from '$lib/stores';
	import { Button, Badge, Card } from '$lib/components/ui';
	import { Laptop, Smartphone, Monitor, Trash2, LogOut, RefreshCw, ShieldCheck } from '@lucide/svelte';

	let sessions = $state<SessionItem[]>([]);
	let isLoading = $state(true);
	let revokingId = $state<string | null>(null);
	let isRevokingOthers = $state(false);

	onMount(async () => {
		await load();
	});

	export async function load(): Promise<void> {
		isLoading = true;
		try {
			sessions = await listSessions();
		} catch {
			addToast('Sitzungen konnten nicht geladen werden', 'error');
		} finally {
			isLoading = false;
		}
	}

	async function handleRevoke(id: string) {
		revokingId = id;
		try {
			await revokeSession(id);
			addToast('Sitzung erfolgreich beendet', 'success');
			await load();
		} catch (err: unknown) {
			addToast(err instanceof Error ? err.message : 'Fehler beim Beenden der Sitzung', 'error');
		} finally {
			revokingId = null;
		}
	}

	async function handleRevokeOthers() {
		isRevokingOthers = true;
		try {
			await revokeOtherSessions();
			addToast('Alle anderen Sitzungen wurden abgemeldet', 'success');
			await load();
		} catch (err: unknown) {
			addToast(err instanceof Error ? err.message : 'Fehler beim Beenden der Sitzungen', 'error');
		} finally {
			isRevokingOthers = false;
		}
	}

	// Kuerzt User-Agent und gibt ein lesbares Geraetelabel zurueck
	function parseDevice(ua: string): { label: string; icon: typeof Laptop } {
		if (!ua) return { label: 'Unbekanntes Gerät', icon: Laptop };
		const lower = ua.toLowerCase();
		let icon = Laptop;
		let label = 'Desktop Browser';

		if (lower.includes('mobile') || lower.includes('android') || lower.includes('iphone')) {
			icon = Smartphone;
			label = 'Mobiles Gerät';
		} else if (lower.includes('macintosh') || lower.includes('mac os')) {
			label = 'Mac';
		} else if (lower.includes('windows')) {
			label = 'Windows PC';
		} else if (lower.includes('linux')) {
			label = 'Linux PC';
		}

		if (lower.includes('firefox')) {
			label += ' (Firefox)';
		} else if (lower.includes('chrome') && !lower.includes('edg')) {
			label += ' (Chrome)';
		} else if (lower.includes('safari') && !lower.includes('chrome')) {
			label += ' (Safari)';
		} else if (lower.includes('edg')) {
			label += ' (Edge)';
		}

		return { label, icon };
	}

	// Kuerzt IP-Adresse fuer kompakte Anzeige
	function formatIp(ip: string): string {
		if (!ip) return '—';
		return ip.replace('::ffff:', '');
	}
</script>

<Card title="Aktive Sitzungen" description="Verwalten Sie alle angemeldeten Geräte und Browser-Sitzungen">
	{#snippet header()}
		<div class="flex flex-col sm:flex-row sm:items-center justify-between gap-3 w-full">
			<div>
				<h3 class="font-semibold text-sm sm:text-base text-text-light dark:text-text-dark leading-none">
					Aktive Sitzungen
				</h3>
				<p class="text-xs text-muted-light dark:text-muted-dark mt-1">
					Geräte mit bestehendem Anmeldestatus (DSGVO Art. 5 Datensparsamkeit)
				</p>
			</div>
			<div class="flex items-center gap-2">
				<Button
					variant="secondary"
					size="sm"
					onclick={load}
					disabled={isLoading}
					title="Aktualisieren"
				>
					<RefreshCw class="w-3.5 h-3.5 {isLoading ? 'animate-spin' : ''}" />
				</Button>
				<Button
					variant="secondary"
					size="sm"
					onclick={handleRevokeOthers}
					loading={isRevokingOthers}
					disabled={sessions.length <= 1}
				>
					<LogOut class="w-3.5 h-3.5 mr-1.5" />
					<span>Alle anderen beenden</span>
				</Button>
			</div>
		</div>
	{/snippet}

	{#if isLoading && sessions.length === 0}
		<div class="py-8 text-center text-xs text-muted-light dark:text-muted-dark space-y-2">
			<RefreshCw class="w-5 h-5 mx-auto animate-spin text-primary" />
			<p>Sitzungen werden geladen...</p>
		</div>
	{:else if sessions.length === 0}
		<div class="py-8 text-center text-xs text-muted-light dark:text-muted-dark">
			<ShieldCheck class="w-8 h-8 mx-auto text-accent mb-2" />
			<p>Keine aktiven Sitzungen registriert.</p>
		</div>
	{:else}
		<div class="overflow-x-auto -mx-4 sm:-mx-6">
			<table class="w-full text-left text-xs border-collapse">
				<thead>
					<tr class="border-b border-border-light dark:border-border-dark bg-slate-50/50 dark:bg-surface-dark/50 text-muted-light dark:text-muted-dark">
						<th class="py-2.5 px-4 sm:px-6 font-medium">Gerät & Browser</th>
						<th class="py-2.5 px-4 font-medium">IP-Adresse</th>
						<th class="py-2.5 px-4 font-medium">Angemeldet am</th>
						<th class="py-2.5 px-4 font-medium">Status</th>
						<th class="py-2.5 px-4 sm:px-6 text-right font-medium">Aktion</th>
					</tr>
				</thead>
				<tbody class="divide-y divide-border-light dark:divide-border-dark">
					{#each sessions as session (session.id)}
						{@const dev = parseDevice(session.user_agent)}
						{@const DevIcon = dev.icon}
						<tr class="hover:bg-slate-50/70 dark:hover:bg-surface-dark/70 transition-colors {session.is_current ? 'bg-primary/5' : ''}">
							<td class="py-3 px-4 sm:px-6">
								<div class="flex items-center gap-3">
									<div class="p-2 rounded-lg bg-slate-100 dark:bg-bg-dark text-primary dark:text-primary-light shrink-0">
										<DevIcon class="w-4 h-4" />
									</div>
									<div class="min-w-0">
										<p class="font-medium text-text-light dark:text-text-dark truncate">
											{dev.label}
										</p>
										<p class="text-2xs text-muted-light dark:text-muted-dark font-mono truncate max-w-xs" title={session.user_agent}>
											{session.user_agent || 'Standard-Client'}
										</p>
									</div>
								</div>
							</td>
							<td class="py-3 px-4 font-mono tabular-nums text-text-light dark:text-text-dark">
								{formatIp(session.ip)}
							</td>
							<td class="py-3 px-4 text-muted-light dark:text-muted-dark tabular-nums">
								{formatDate(session.created_at)}
							</td>
							<td class="py-3 px-4">
								{#if session.is_current}
									<Badge variant="primary" size="sm">Aktuelle Sitzung</Badge>
								{:else}
									<Badge variant="neutral" size="sm">Aktiv</Badge>
								{/if}
							</td>
							<td class="py-3 px-4 sm:px-6 text-right">
								{#if session.is_current}
									<span class="text-2xs text-muted-light dark:text-muted-dark italic">Dieses Gerät</span>
								{:else}
									<Button
										variant="ghost"
										size="sm"
										onclick={() => handleRevoke(session.id)}
										loading={revokingId === session.id}
										title="Sitzung beenden"
										aria-label="Sitzung beenden"
									>
										<Trash2 class="w-3.5 h-3.5 text-danger" />
									</Button>
								{/if}
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</Card>

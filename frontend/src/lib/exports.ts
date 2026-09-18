import { api } from './api';

// Status eines DSGVO-Datenexport-Auftrags (Art. 20 DSGVO)
export interface ExportJobStatus {
	job_id: string;
	status: 'pending' | 'processing' | 'completed' | 'failed' | 'expired';
	download_url?: string;
	expires_at?: string;
	size_bytes?: number;
}

// Startet einen neuen asynchronen Datenexport-Auftrag mit Passwort-Schutz
export async function requestExport(password: string): Promise<{ job_id: string; status: string }> {
	return await api<{ job_id: string; status: string }>('/users/me/export', {
		method: 'POST',
		body: JSON.stringify({ password })
	});
}

// Ermittelt den aktuellen Verarbeitungsstatus eines Export-Auftrags
export async function getExportStatus(jobId: string): Promise<ExportJobStatus> {
	return await api<ExportJobStatus>(`/users/me/export/${jobId}`);
}

// Liefert die Download-URL fuer ein fertiggestelltes Export-Archiv
export function getExportDownloadUrl(jobId: string): string {
	return `/api/v1/users/me/export/${jobId}/download`;
}

import { api } from './api';

export interface FileItem {
	id: string;
	filename: string;
	size_bytes: number;
	mime_type: string;
	thumbnail_path?: string;
	width?: number;
	height?: number;
	taken_at?: string;
	created_at: string;
}

export interface FileListResponse {
	files: FileItem[];
	total: number;
	page: number;
	limit: number;
}

export interface ShareResponse {
	share_token: string;
	share_url: string;
	expires_at: string;
}

// Formatierungshelfer fuer Dateigroessen
export function formatBytes(bytes: number, decimals = 1): string {
	if (bytes === 0) return '0 B';
	const k = 1024;
	const dm = decimals < 0 ? 0 : decimals;
	const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
	const i = Math.floor(Math.log(bytes) / Math.log(k));
	return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i];
}

// Formatierungshelfer fuer ISO-Zeitstempel
export function formatDate(dateString: string): string {
	try {
		const d = new Date(dateString);
		return new Intl.DateTimeFormat('de-DE', {
			day: '2-digit',
			month: '2-digit',
			year: 'numeric',
			hour: '2-digit',
			minute: '2-digit'
		}).format(d);
	} catch {
		return dateString;
	}
}

// Formatierungshelfer fuer relative Zeitangaben ("vor 2 Stunden")
export function formatRelativeTime(dateString: string): string {
	try {
		const now = Date.now();
		const d = new Date(dateString).getTime();
		const diffSec = Math.max(0, Math.floor((now - d) / 1000));
		if (diffSec < 60) return 'gerade eben';
		const diffMin = Math.floor(diffSec / 60);
		if (diffMin < 60) return `vor ${diffMin} ${diffMin === 1 ? 'Minute' : 'Minuten'}`;
		const diffHours = Math.floor(diffMin / 60);
		if (diffHours < 24) return `vor ${diffHours} ${diffHours === 1 ? 'Stunde' : 'Stunden'}`;
		const diffDays = Math.floor(diffHours / 24);
		if (diffDays < 30) return `vor ${diffDays} ${diffDays === 1 ? 'Tag' : 'Tagen'}`;
		const diffMonths = Math.floor(diffDays / 30);
		if (diffMonths < 12) return `vor ${diffMonths} ${diffMonths === 1 ? 'Monat' : 'Monaten'}`;
		return `vor ${Math.floor(diffMonths / 12)} Jahren`;
	} catch {
		return 'kürzlich';
	}
}

// Ruft die Dateiliste mit Pagination, Filter und Sortierung ab
export async function listFiles(
	page = 1,
	limit = 50,
	sort = 'date',
	order: 'asc' | 'desc' = 'desc',
	filter = ''
): Promise<FileListResponse> {
	const params = new URLSearchParams({
		page: String(page),
		limit: String(limit),
		sort,
		order
	});
	if (filter.trim()) {
		params.set('filter', filter.trim());
	}

	return await api<FileListResponse>(`/files?${params.toString()}`);
}

// Fordert eine signierte Presigned-Download-URL an
export async function downloadFile(id: string): Promise<{ download_url: string; filename: string }> {
	return await api<{ download_url: string; filename: string }>(`/files/${id}/download`);
}

// Loescht eine Datei physisch und aus der Datenbank
export async function deleteFile(id: string): Promise<void> {
	await api(`/files/${id}`, {
		method: 'DELETE'
	});
}

// Benennt eine Datei um
export async function renameFile(id: string, newName: string): Promise<{ id: string; filename: string }> {
	return await api<{ id: string; filename: string }>(`/files/${id}`, {
		method: 'PATCH',
		body: JSON.stringify({ filename: newName.trim() })
	});
}

// Erstellt einen sicheren Freigabelink
export async function createShare(
	id: string,
	options: { expires_days?: number; password?: string } = {}
): Promise<ShareResponse> {
	return await api<ShareResponse>(`/files/${id}/share`, {
		method: 'POST',
		body: JSON.stringify(options)
	});
}

export interface QuotaResponse {
	used_bytes: number;
	total_bytes: number;
	percent: number;
}

// Ruft den Speicherplatzverbrauch und das Quota des Nutzers ab
export async function getQuota(): Promise<QuotaResponse> {
	return await api<QuotaResponse>('/users/me/quota');
}


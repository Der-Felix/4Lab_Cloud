import { api } from './api';

export interface SessionItem {
	id: string;
	ip: string;
	user_agent: string;
	created_at: string;
	is_current?: boolean;
}

export interface ExportJobStatus {
	job_id: string;
	status: 'pending' | 'processing' | 'completed' | 'failed';
	download_url?: string;
	expires_at?: string;
}

export async function listSessions(): Promise<SessionItem[]> {
	try {
		const res = await api<{ sessions: SessionItem[] }>('/auth/sessions');
		return res.sessions || [];
	} catch {
		return [];
	}
}

export async function revokeSession(id: string): Promise<void> {
	await api(`/auth/sessions/${id}`, { method: 'DELETE' });
}

export async function revokeOtherSessions(): Promise<void> {
	await api('/auth/sessions', { method: 'DELETE' });
}

export async function requestExport(password: string): Promise<{ job_id: string; status: string }> {
	return await api<{ job_id: string; status: string }>('/users/me/export', {
		method: 'POST',
		body: JSON.stringify({ password })
	});
}

export async function getExportStatus(jobId: string): Promise<ExportJobStatus> {
	return await api<ExportJobStatus>(`/users/me/export/${jobId}`);
}

export async function deleteAccount(password: string, totpCode?: string): Promise<void> {
	await api('/users/me', {
		method: 'DELETE',
		body: JSON.stringify({ password, totp_code: totpCode })
	});
}

export async function inviteUser(email: string, role = 'user'): Promise<{ token: string; expires_at: string }> {
	return await api<{ token: string; expires_at: string }>('/auth/invite', {
		method: 'POST',
		body: JSON.stringify({ email, role })
	});
}

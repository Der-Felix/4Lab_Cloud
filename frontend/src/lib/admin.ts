import { api } from './api';

// Benutzerdatensatz fuer die administrative Nutzerverwaltung
export interface AdminUserItem {
	id: string;
	email: string;
	is_admin: boolean;
	mfa_enabled: boolean;
	created_at: string;
}

// Einladung fuer einen neuen Benutzer
export interface InvitationItem {
	id: string;
	email: string;
	created_at: string;
	expires_at: string;
}

// Rueckgabe bei Erstellung einer neuen Einladung
export interface InviteResponse {
	invitation_token: string;
	email: string;
	expires_at: string;
}

// Audit-Log-Eintrag (DSGVO Art. 32 / BSI TR-02102)
export interface AuditLogItem {
	id: number;
	user_id?: string;
	user_email?: string;
	pseudonym_hash?: string;
	action: string;
	target_id?: string;
	ip_address: string;
	result: string;
	created_at: string;
}

// Ruft die Liste aller registrierten Benutzer ab (nur fuer Administratoren)
export async function listUsers(): Promise<AdminUserItem[]> {
	try {
		const res = await api<{ users: AdminUserItem[] }>('/admin/users');
		return res.users || [];
	} catch {
		return [];
	}
}

// Loescht einen Benutzer als Administrator (DSGVO-konforme Kaskadenloeschung)
export async function deleteUser(id: string): Promise<void> {
	await api(`/admin/users/${id}`, { method: 'DELETE' });
}

// Erstellt eine neue Registrierungs-Einladung (nur fuer Administratoren)
export async function inviteUser(email: string, role = 'user'): Promise<InviteResponse> {
	return await api<InviteResponse>('/auth/invite', {
		method: 'POST',
		body: JSON.stringify({ email, role })
	});
}

// Ruft alle noch gueltigen, ungenutzten Einladungen ab
export async function listInvitations(): Promise<InvitationItem[]> {
	try {
		const res = await api<{ invitations: InvitationItem[] }>('/admin/invitations');
		return res.invitations || [];
	} catch {
		return [];
	}
}

// Ruft die letzten 100 Audit-Log-Eintraege mit optionalem Filter ab
export async function listAuditLogs(action = '', user = ''): Promise<AuditLogItem[]> {
	try {
		const params = new URLSearchParams();
		if (action) params.set('action', action);
		if (user) params.set('user', user);
		const qs = params.toString();
		const endpoint = qs ? `/admin/audit?${qs}` : '/admin/audit';
		const res = await api<{ logs: AuditLogItem[] }>(endpoint);
		return res.logs || [];
	} catch {
		return [];
	}
}

// Ruft die persoenlichen Audit-Logs des angemeldeten Benutzers ab
export async function getMyAuditLogs(): Promise<AuditLogItem[]> {
	try {
		const res = await api<{ logs: AuditLogItem[] }>('/audit/me');
		return res.logs || [];
	} catch {
		return [];
	}
}


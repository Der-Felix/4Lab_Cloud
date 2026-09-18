import { api } from './api';

// Repraesentiert eine aktive Benutzersitzung (Refresh-Token)
export interface SessionItem {
	id: string;
	ip: string;
	user_agent: string;
	created_at: string;
	expires_at?: string;
	is_current?: boolean;
}

// Ruft alle aktiven Sitzungen des aktuellen Benutzers ab
export async function listSessions(): Promise<SessionItem[]> {
	try {
		const res = await api<{ sessions: SessionItem[] }>('/auth/sessions');
		return res.sessions || [];
	} catch {
		return [];
	}
}

// Beendet eine spezifische Sitzung anhand ihrer ID
export async function revokeSession(id: string): Promise<void> {
	await api(`/auth/sessions/${id}`, { method: 'DELETE' });
}

// Beendet alle anderen Sitzungen ausser der aktuellen
export async function revokeOtherSessions(): Promise<void> {
	await api('/auth/sessions', { method: 'DELETE' });
}

import { api } from './api';

export interface ShareItem {
	id: string;
	file_id: string;
	filename: string;
	size_bytes: number;
	token: string;
	share_url: string;
	has_password: boolean;
	expires_at: string | null;
	created_at: string;
}

// Ruft alle aktiven Freigaben des Benutzers ab
export async function listShares(): Promise<ShareItem[]> {
	const res = await api<{ shares: ShareItem[] }>('/shares');
	return res.shares || [];
}

// Widerruft eine Freigabe
export async function deleteShare(id: string): Promise<void> {
	await api(`/shares/${id}`, { method: 'DELETE' });
}

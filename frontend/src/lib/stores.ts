import { writable, derived } from 'svelte/store';

// Benutzerdaten gemaess DSGVO Art. 5 (nur notwendige Metadaten im UI-Memory)
export interface UserInfo {
	id: string;
	email: string;
	isAdmin: boolean;
}

// Access-Token liegt strikt im Arbeitsspeicher (niemals in localStorage oder sessionStorage)
export const accessToken = writable<string | null>(null);

// Aktueller Benutzerstatus
export const currentUser = writable<UserInfo | null>(null);

// Statusanzeige, ob der initiale Session-Check durchgefuehrt wurde
export const authInitialized = writable<boolean>(false);

// Temporaerer MFA-Token fuer 2FA-Schritte (liegt ebenfalls rein im Memory)
export const pendingMfaToken = writable<string | null>(null);

// Abgeleiteter Status fuer eingeloggten Zustand
export const isAuthenticated = derived(
	[accessToken, currentUser],
	([$token, $user]) => Boolean($token && $user)
);

// Hilfsfunktion zum vollstaendigen Zuruecksetzen des Auth-States
export function clearAuthState(): void {
	accessToken.set(null);
	currentUser.set(null);
	pendingMfaToken.set(null);
}

// Toast-Benachrichtigungssystem
export interface Toast {
	id: string;
	message: string;
	type: 'success' | 'error' | 'info';
	duration?: number;
}

export const toasts = writable<Toast[]>([]);

export function addToast(message: string, type: 'success' | 'error' | 'info' = 'info', duration = 4000): void {
	const id = Math.random().toString(36).substring(2, 9);
	toasts.update((all) => [...all, { id, message, type, duration }]);
	if (duration > 0) {
		setTimeout(() => {
			removeToast(id);
		}, duration);
	}
}

export function removeToast(id: string): void {
	toasts.update((all) => all.filter((t) => t.id !== id));
}


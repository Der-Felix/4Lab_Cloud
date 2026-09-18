import { get } from 'svelte/store';
import { api, refreshAccessToken } from './api';
import { accessToken, currentUser, pendingMfaToken, clearAuthState, authInitialized, type UserInfo } from './stores';

// Dekodiert den JWT-Payload sicher im Speicher (ohne externe Abhaengigkeit)
export function parseJwtPayload(token: string): { sub?: string; exp?: number; [key: string]: unknown } | null {
	try {
		const parts = token.split('.');
		if (parts.length !== 3) return null;
		const base64 = parts[1].replace(/-/g, '+').replace(/_/g, '/');
		const jsonPayload = decodeURIComponent(
			atob(base64)
				.split('')
				.map((c) => '%' + ('00' + c.charCodeAt(0).toString(16)).slice(-2))
				.join('')
		);
		return JSON.parse(jsonPayload);
	} catch {
		return null;
	}
}

export interface LoginResult {
	success: boolean;
	mfaRequired?: boolean;
	user?: UserInfo;
}

// Timer fuer den automatischen Token-Refresh vor Ablauf
let autoRefreshTimer: ReturnType<typeof setTimeout> | null = null;

function scheduleAutoRefresh(expiresInSeconds = 900): void {
	if (autoRefreshTimer) {
		clearTimeout(autoRefreshTimer);
		autoRefreshTimer = null;
	}
	// Refresh ca. 2 Minuten vor Ablauf ausfuehren (mindestens nach 30s)
	const delayMs = Math.max((expiresInSeconds - 120) * 1000, 30 * 1000);
	autoRefreshTimer = setTimeout(async () => {
		const newToken = await refreshAccessToken();
		if (newToken) {
			const payload = parseJwtPayload(newToken);
			const ttl = payload?.exp ? Math.max(payload.exp - Math.floor(Date.now() / 1000), 60) : 900;
			scheduleAutoRefresh(ttl);
		}
	}, delayMs);
}

// Meldet den Benutzer mit E-Mail und Passwort an
export async function login(email: string, password: string): Promise<LoginResult> {
	const res = await api<{
		access_token?: string;
		mfa_token?: string;
		mfa_required?: boolean;
		expires_in?: number;
		is_admin?: boolean;
	}>('/auth/login', {
		method: 'POST',
		body: JSON.stringify({ email, password }),
		skipAuth: true
	});

	if (res.mfa_required) {
		pendingMfaToken.set(res.mfa_token || null);
		return { success: false, mfaRequired: true };
	}


	if (res.access_token) {
		accessToken.set(res.access_token);
		const payload = parseJwtPayload(res.access_token);
		const user: UserInfo = {
			id: payload?.sub || '',
			email,
			isAdmin: Boolean(res.is_admin)
		};
		currentUser.set(user);
		scheduleAutoRefresh(res.expires_in || 900);
		return { success: true, user };
	}

	return { success: false };
}

// Bestaetigt den 2. Faktor (TOTP)
export async function verifyMfa(code: string): Promise<boolean> {
	const mfaToken = get(pendingMfaToken);
	if (!mfaToken) {
		throw new Error('Keine offene MFA-Sitzung vorhanden');
	}

	const res = await api<{
		access_token: string;
		expires_in?: number;
		is_admin?: boolean;
	}>('/auth/mfa/verify', {
		method: 'POST',
		body: JSON.stringify({ mfa_token: mfaToken, code }),
		skipAuth: true
	});

	if (res.access_token) {
		accessToken.set(res.access_token);
		pendingMfaToken.set(null);
		const payload = parseJwtPayload(res.access_token);
		currentUser.set({
			id: payload?.sub || '',
			email: '',
			isAdmin: Boolean(res.is_admin)
		});
		scheduleAutoRefresh(res.expires_in || 900);
		return true;
	}

	return false;
}

// Verifiziert einen Recovery-Code bei Geraeteverlust
export async function verifyMfaRecovery(recoveryCode: string): Promise<boolean> {
	const mfaToken = get(pendingMfaToken);
	if (!mfaToken) {
		throw new Error('Keine offene MFA-Sitzung vorhanden');
	}

	const res = await api<{
		access_token: string;
		expires_in?: number;
		is_admin?: boolean;
	}>('/auth/mfa/verify-recovery', {
		method: 'POST',
		body: JSON.stringify({ mfa_token: mfaToken, recovery_code: recoveryCode }),
		skipAuth: true
	});

	if (res.access_token) {
		accessToken.set(res.access_token);
		pendingMfaToken.set(null);
		const payload = parseJwtPayload(res.access_token);
		currentUser.set({
			id: payload?.sub || '',
			email: '',
			isAdmin: Boolean(res.is_admin)
		});
		scheduleAutoRefresh(res.expires_in || 900);
		return true;
	}

	return false;
}

// Registriert einen Benutzer (Erstnutzer wird Admin; sonst nur mit Einladungstoken)
export async function registerUser(email: string, password: string, invitationToken?: string): Promise<{ user_id: string; email: string; is_admin: boolean }> {
	return await api<{ user_id: string; email: string; is_admin: boolean }>('/auth/register', {
		method: 'POST',
		body: JSON.stringify({
			email,
			password,
			invitation_token: invitationToken || ''
		}),
		skipAuth: true
	});
}

// Initialisiert MFA-Setup und gibt Secret + otpauth URI zurueck
export async function setupMfa(): Promise<{ secret: string; qr_uri: string }> {
	const mfaToken = get(pendingMfaToken);
	return await api<{ secret: string; qr_uri: string }>('/auth/mfa/setup', {
		method: 'POST',
		body: JSON.stringify(mfaToken ? { mfa_token: mfaToken } : {})
	});
}

// Aktiviert MFA mit dem ersten Bestaetigungscode und liefert 10 Recovery-Codes
export async function activateMfa(code: string): Promise<{ recovery_codes: string[] }> {
	const mfaToken = get(pendingMfaToken);
	const res = await api<{
		recovery_codes: string[];
		access_token?: string;
		is_admin?: boolean;
		expires_in?: number;
	}>('/auth/mfa/activate', {
		method: 'POST',
		body: JSON.stringify({
			code,
			mfa_token: mfaToken || ''
		})
	});

	if (res.access_token) {
		accessToken.set(res.access_token);
		pendingMfaToken.set(null);
		const payload = parseJwtPayload(res.access_token);
		currentUser.set({
			id: payload?.sub || '',
			email: '',
			isAdmin: Boolean(res.is_admin)
		});
		scheduleAutoRefresh(res.expires_in || 900);
	}

	return { recovery_codes: res.recovery_codes };
}

// Deaktiviert MFA mit TOTP- oder Recovery-Code
export async function deactivateMfa(code: string): Promise<void> {
	await api('/auth/mfa/deactivate', {
		method: 'POST',
		body: JSON.stringify({ code })
	});
}

// Loggt den Benutzer aus, loescht Cookies und invalidiert State
export async function logout(): Promise<void> {
	if (autoRefreshTimer) {
		clearTimeout(autoRefreshTimer);
		autoRefreshTimer = null;
	}

	try {
		await api('/auth/logout', {
			method: 'POST',
			body: JSON.stringify({})
		});
	} catch {
		// Fehler beim Server-Logout ignorieren, lokaler State wird dennoch bereinigt
	} finally {
		clearAuthState();
	}
}

// Versucht beim ersten Laden der App, die Session via Cookie-Refresh wiederherzustellen
export async function initializeSession(): Promise<boolean> {
	try {
		const token = await refreshAccessToken();
		if (token) {
			const payload = parseJwtPayload(token);
			if (payload?.sub) {
				currentUser.update((u) => ({
					id: payload.sub as string,
					email: u?.email || (payload?.email as string) || '',
					isAdmin: Boolean(payload?.is_admin ?? u?.isAdmin)
				}));
				scheduleAutoRefresh(900);
				return true;
			}
		}
		return false;
	} finally {
		authInitialized.set(true);
	}
}

// Prueft, ob der aktuell angemeldete Benutzer Administrator-Rechte besitzt
export function isAdmin(): boolean {
	const user = get(currentUser);
	if (user?.isAdmin) return true;
	const token = get(accessToken);
	if (token) {
		const payload = parseJwtPayload(token);
		if (payload?.is_admin === true || payload?.role === 'admin') return true;
	}
	return false;
}

// Loescht Auth-Cookies clientseitig nach Account-Loeschung oder Logout
export function clearAuthCookies(): void {
	if (typeof document !== 'undefined') {
		document.cookie = 'refresh_token=; path=/api/v1/auth; expires=Thu, 01 Jan 1970 00:00:00 GMT; SameSite=Strict; Secure';
		document.cookie = 'csrf_token=; path=/; expires=Thu, 01 Jan 1970 00:00:00 GMT; SameSite=Strict; Secure';
	}
}


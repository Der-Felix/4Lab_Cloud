import { get } from 'svelte/store';
import { accessToken, currentUser, clearAuthState } from './stores';

export class ApiError extends Error {
	public status: number;
	public data: unknown;

	constructor(status: number, message: string, data?: unknown) {
		super(message);
		this.name = 'ApiError';
		this.status = status;
		this.data = data;
	}
}

const API_BASE = '/api/v1';

// Liest das csrf_token Cookie synchron aus document.cookie
export function getCsrfToken(): string | null {
	if (typeof document === 'undefined') {
		return null;
	}
	const cookies = document.cookie.split(';');
	for (const rawCookie of cookies) {
		const cookie = rawCookie.trim();
		if (cookie.startsWith('csrf_token=')) {
			return decodeURIComponent(cookie.substring('csrf_token='.length));
		}
	}
	return null;
}

// Mutex / Promise fuer gleichzeitige Refresh-Vorgaenge
let refreshPromise: Promise<string | null> | null = null;

// Fuehrt den Token-Refresh durch (POST /api/v1/auth/refresh)
export async function refreshAccessToken(): Promise<string | null> {
	if (refreshPromise) {
		return refreshPromise;
	}

	refreshPromise = (async () => {
		try {
			const csrfToken = getCsrfToken();
			const headers: Record<string, string> = {
				'Content-Type': 'application/json'
			};
			if (csrfToken) {
				headers['X-CSRF-Token'] = csrfToken;
			}

			const res = await fetch(`${API_BASE}/auth/refresh`, {
				method: 'POST',
				credentials: 'include',
				headers,
				body: JSON.stringify({})
			});

			if (!res.ok) {
				clearAuthState();
				return null;
			}

			const data = await res.json();
			if (data.access_token) {
				accessToken.set(data.access_token);
				currentUser.update((u) => ({
					id: u?.id || '',
					email: data.email || u?.email || '',
					isAdmin: Boolean(data.is_admin ?? u?.isAdmin)
				}));
				return data.access_token as string;
			}

			clearAuthState();
			return null;
		} catch {
			clearAuthState();
			return null;
		} finally {
			refreshPromise = null;
		}
	})();

	return refreshPromise;
}

interface RequestOptions extends RequestInit {
	skipAuth?: boolean;
	retryOnUnauthorized?: boolean;
}

// Zentraler Handler fuer API-Anfragen mit Auth-Header, CSRF und Refresh-Deduplizierung
async function executeRequest(url: string, options: RequestOptions = {}): Promise<Response> {
	const { skipAuth = false, retryOnUnauthorized = true, headers: customHeaders = {}, ...restInit } = options;

	// Falls gerade ein Token-Refresh aktiv ist, auf dessen Abschluss warten
	if (!skipAuth && refreshPromise) {
		await refreshPromise;
	}

	const headers: Record<string, string> = {
		...(customHeaders as Record<string, string>)
	};

	// CSRF-Token bei mutationsfaehigen Anfragen anhaengen falls verfuegbar
	const csrfToken = getCsrfToken();
	if (csrfToken && !headers['X-CSRF-Token']) {
		headers['X-CSRF-Token'] = csrfToken;
	}

	// Bearer Access-Token aus dem Store anhaengen und merken
	let sentToken: string | null = null;
	if (!skipAuth) {
		sentToken = get(accessToken);
		if (sentToken && !headers['Authorization']) {
			headers['Authorization'] = `Bearer ${sentToken}`;
		}
	}

	let response = await fetch(url, {
		...restInit,
		credentials: 'include',
		headers
	});

	// Bei 401 Unauthorized Retry pruefen
	if (
		response.status === 401 &&
		retryOnUnauthorized &&
		!url.includes('/auth/login') &&
		!url.includes('/auth/refresh')
	) {
		// Pruefen, ob ein paralleler Request den Token bereits aktualisiert hat
		const currentToken = get(accessToken);
		let newToken: string | null = null;

		if (currentToken && currentToken !== sentToken) {
			// Token wurde bereits von einem parallelen Refresh erneuert
			newToken = currentToken;
		} else {
			// Token muss erneuert werden (oder wir haengen uns an das laufende refreshPromise)
			newToken = await refreshAccessToken();
		}

		if (newToken) {
			headers['Authorization'] = `Bearer ${newToken}`;
			const newCsrf = getCsrfToken();
			if (newCsrf) {
				headers['X-CSRF-Token'] = newCsrf;
			}

			response = await fetch(url, {
				...restInit,
				credentials: 'include',
				headers
			});
		} else {
			// Refresh fehlgeschlagen: Weiterleitung zum Login im Browser
			if (typeof window !== 'undefined' && !window.location.pathname.startsWith('/login')) {
				window.location.href = '/login';
			}
		}
	}

	return response;
}

// Zentraler API Fetch-Wrapper fuer JSON-Antworten
export async function api<T = unknown>(path: string, options: RequestOptions = {}): Promise<T> {
	const url = path.startsWith('http') ? path : `${API_BASE}${path.startsWith('/') ? path : `/${path}`}`;
	const headers = {
		'Content-Type': 'application/json',
		...(options.headers as Record<string, string>)
	};

	const response = await executeRequest(url, {
		...options,
		headers
	});

	if (!response.ok) {
		let errorMessage = `HTTP-Fehler ${response.status}`;
		let errorData: unknown = null;
		try {
			const body = await response.json();
			errorData = body;
			if (body && typeof body === 'object' && 'error' in body) {
				errorMessage = String(body.error);
			}
		} catch {
			try {
				const text = await response.text();
				if (text) errorMessage = text;
			} catch {
				// Fallback auf Standardnachricht
			}
		}

		throw new ApiError(response.status, errorMessage, errorData);
	}

	if (response.status === 204) {
		return null as T;
	}

	return (await response.json()) as T;
}

// Zentraler API Fetch-Wrapper fuer Binaerdaten (z. B. Thumbnails mit JWT-Auth)
export async function apiBlob(path: string, options: RequestOptions = {}): Promise<Blob> {
	const url = path.startsWith('http') ? path : `${API_BASE}${path.startsWith('/') ? path : `/${path}`}`;
	const response = await executeRequest(url, options);

	if (!response.ok) {
		let errorMessage = `HTTP-Fehler ${response.status}`;
		let errorData: unknown = null;
		try {
			const body = await response.json();
			errorData = body;
			if (body && typeof body === 'object' && 'error' in body) {
				errorMessage = String(body.error);
			}
		} catch {
			try {
				const text = await response.text();
				if (text) errorMessage = text;
			} catch {
				// Fallback auf Standardnachricht
			}
		}

		throw new ApiError(response.status, errorMessage, errorData);
	}

	return await response.blob();
}

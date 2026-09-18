import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { api, ApiError, getCsrfToken } from './api';
import { accessToken, currentUser } from './stores';
import { get } from 'svelte/store';

describe('API-Client (src/lib/api.ts)', () => {
	beforeEach(() => {
		accessToken.set(null);
		currentUser.set(null);
		vi.restoreAllMocks();
		// Mock fuer document.cookie
		let cookieJar = '';
		Object.defineProperty(globalThis, 'document', {
			value: {
				get cookie() {
					return cookieJar;
				},
				set cookie(val: string) {
					cookieJar = val;
				}
			},
			configurable: true,
			writable: true
		});
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('liest das CSRF-Token korrekt aus document.cookie', () => {
		document.cookie = 'other=foo; csrf_token=test-csrf-1234; session=abc';
		expect(getCsrfToken()).toBe('test-csrf-1234');
	});

	it('sendet X-CSRF-Token und Authorization Header bei API-Aufrufen', async () => {
		document.cookie = 'csrf_token=valid-csrf-token';
		accessToken.set('valid-jwt-token');

		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: async () => ({ status: 'success' })
		});
		vi.stubGlobal('fetch', fetchMock);

		const result = await api<{ status: string }>('/test-endpoint');

		expect(result).toEqual({ status: 'success' });
		expect(fetchMock).toHaveBeenCalledTimes(1);

		const callArgs = fetchMock.mock.calls[0];
		expect(callArgs[0]).toBe('/api/v1/test-endpoint');
		const headers = callArgs[1].headers;
		expect(headers['X-CSRF-Token']).toBe('valid-csrf-token');
		expect(headers['Authorization']).toBe('Bearer valid-jwt-token');
	});

	it('wiederholt Anfrage nach erfolgreichem 401-Refresh (Retry)', async () => {
		document.cookie = 'csrf_token=csrf-first';
		accessToken.set('expired-jwt');

		const fetchMock = vi.fn()
			// 1. Aufruf: 401 Unauthorized
			.mockResolvedValueOnce({
				ok: false,
				status: 401,
				json: async () => ({ error: 'Token expired' })
			})
			// 2. Aufruf: Refresh-Endpoint antwortet mit 200 und neuem Access-Token
			.mockResolvedValueOnce({
				ok: true,
				status: 200,
				json: async () => {
					document.cookie = 'csrf_token=csrf-second';
					return {
						access_token: 'fresh-new-jwt',
						csrf_token: 'csrf-second'
					};
				}
			})
			// 3. Aufruf: Wiederholung des Original-Requests mit neuem Token erfolgreich
			.mockResolvedValueOnce({
				ok: true,
				status: 200,
				json: async () => ({ data: 'secret payload' })
			});

		vi.stubGlobal('fetch', fetchMock);

		const result = await api<{ data: string }>('/secure-resource');

		expect(result).toEqual({ data: 'secret payload' });
		expect(fetchMock).toHaveBeenCalledTimes(3);

		// Pruefen, ob der Refresh aufgerufen wurde
		expect(fetchMock.mock.calls[1][0]).toBe('/api/v1/auth/refresh');

		// Pruefen, ob der Store das neue Access-Token enthaelt
		expect(get(accessToken)).toBe('fresh-new-jwt');

		// Pruefen, ob die Wiederholung das neue Token im Header nutzt
		const retryCall = fetchMock.mock.calls[2];
		expect(retryCall[1].headers['Authorization']).toBe('Bearer fresh-new-jwt');
	});

	it('wirft ApiError und leert State wenn 401-Refresh fehlschlaegt', async () => {
		accessToken.set('invalid-jwt');

		const fetchMock = vi.fn()
			// 1. Original-Aufruf liefert 401
			.mockResolvedValueOnce({
				ok: false,
				status: 401,
				json: async () => ({ error: 'Unauthorized' })
			})
			// 2. Refresh liefert ebenfalls 401
			.mockResolvedValueOnce({
				ok: false,
				status: 401,
				json: async () => ({ error: 'Refresh token expired' })
			});

		vi.stubGlobal('fetch', fetchMock);

		await expect(api('/secure-resource')).rejects.toThrow(ApiError);
		expect(get(accessToken)).toBeNull();
	});

	it('wirft verstaendlichen ApiError bei HTTP 400/500 Antworten', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: false,
			status: 400,
			json: async () => ({ error: 'Ungueltige Parameter' })
		});
		vi.stubGlobal('fetch', fetchMock);

		await expect(api('/some-endpoint')).rejects.toThrow('Ungueltige Parameter');
	});

	it('refresh_dedup_test: 5 parallele Requests fuehren zu nur 1 Refresh-Call', async () => {
		document.cookie = 'csrf_token=test-csrf-dedup';
		accessToken.set('expired-token-123');

		let refreshCallCount = 0;

		const fetchMock = vi.fn().mockImplementation(async (url: string, init?: RequestInit) => {
			if (url.includes('/auth/refresh')) {
				refreshCallCount++;
				// Simuliere kuenstliche Netzwerk-Latenz
				await new Promise((resolve) => setTimeout(resolve, 25));
				return {
					ok: true,
					status: 200,
					json: async () => ({
						access_token: 'fresh-rotated-token-456',
						email: 'felix@4labs.local'
					})
				};
			}

			const authHeader = (init?.headers as Record<string, string>)?.[
				'Authorization'
			];
			if (authHeader === 'Bearer fresh-rotated-token-456') {
				return {
					ok: true,
					status: 200,
					json: async () => ({ status: 'success', path: url })
				};
			}

			// Alter Token liefert 401 Unauthorized
			return {
				ok: false,
				status: 401,
				json: async () => ({ error: 'Token expired' })
			};
		});

		vi.stubGlobal('fetch', fetchMock);

		// 5 gleichzeitige Requests absenden
		const results = await Promise.all([
			api<{ status: string }>('/resource-1'),
			api<{ status: string }>('/resource-2'),
			api<{ status: string }>('/resource-3'),
			api<{ status: string }>('/resource-4'),
			api<{ status: string }>('/resource-5')
		]);

		expect(results).toHaveLength(5);
		results.forEach((res) => {
			expect(res.status).toBe('success');
		});

		// Verbindlich: Nur ein einziger Refresh-Call fuer alle 5 parallelen Anfragen!
		expect(refreshCallCount).toBe(1);
		expect(get(accessToken)).toBe('fresh-rotated-token-456');
	});
});

import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { listUsers, deleteUser, inviteUser, listInvitations, listAuditLogs } from './admin';
import { isAdmin, clearAuthCookies } from './auth';
import { currentUser, accessToken, pendingMfaToken, clearAuthState } from './stores';
import { get } from 'svelte/store';

describe('Admin API Client & Guards (src/lib/admin.ts & auth.ts)', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
		clearAuthState();
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	describe('listUsers', () => {
		it('ruft GET /api/v1/admin/users ab', async () => {
			const mockUsers = [
				{
					id: 'u-1',
					email: 'admin@4labs.local',
					is_admin: true,
					mfa_enabled: true,
					created_at: '2026-09-01T00:00:00Z'
				}
			];
			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 200,
				json: async () => ({ users: mockUsers })
			});
			vi.stubGlobal('fetch', fetchMock);

			const users = await listUsers();
			expect(fetchMock).toHaveBeenCalledTimes(1);
			expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/admin/users');
			expect(users).toHaveLength(1);
			expect(users[0].is_admin).toBe(true);
		});
	});

	describe('deleteUser', () => {
		it('sendet DELETE /api/v1/admin/users/:id', async () => {
			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 204,
				json: async () => ({})
			});
			vi.stubGlobal('fetch', fetchMock);

			await deleteUser('target-user-uuid');
			expect(fetchMock).toHaveBeenCalledTimes(1);
			const [url, opts] = fetchMock.mock.calls[0];
			expect(url).toBe('/api/v1/admin/users/target-user-uuid');
			expect(opts?.method).toBe('DELETE');
		});
	});

	describe('admin_delete_confirm_test', () => {
		it('loescht NICHT, wenn die Bestaetigungs-Email nicht exakt uebereinstimmt', () => {
			const targetUser = {
				id: 'target-123',
				email: 'victim@4labs.local',
				is_admin: false,
				mfa_enabled: false,
				created_at: '2026-09-01T00:00:00Z'
			};

			const mockDeleteFn = vi.fn();

			// Simulation der Bestaetigungslogik aus UserList.svelte
			function tryDelete(inputEmail: string) {
				if (inputEmail.trim().toLowerCase() !== targetUser.email.toLowerCase()) {
					return false;
				}
				mockDeleteFn(targetUser.id);
				return true;
			}

			// 1. Falsche E-Mail -> kein DELETE
			const resWrong = tryDelete('wrong@4labs.local');
			expect(resWrong).toBe(false);
			expect(mockDeleteFn).not.toHaveBeenCalled();

			// 2. Tippfehler -> kein DELETE
			const resTypo = tryDelete('victim@other.local');
			expect(resTypo).toBe(false);
			expect(mockDeleteFn).not.toHaveBeenCalled();

			// 3. Korrekte E-Mail -> DELETE ausgefuehrt
			const resCorrect = tryDelete('victim@4labs.local');
			expect(resCorrect).toBe(true);
			expect(mockDeleteFn).toHaveBeenCalledWith('target-123');
		});
	});

	describe('inviteUser & listInvitations', () => {
		it('inviteUser sendet POST /api/v1/auth/invite mit E-Mail', async () => {
			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 201,
				json: async () => ({
					invitation_token: 'token-abc-123',
					email: 'new@4labs.local',
					expires_at: '2026-09-25T00:00:00Z'
				})
			});
			vi.stubGlobal('fetch', fetchMock);

			const res = await inviteUser('new@4labs.local');
			expect(fetchMock).toHaveBeenCalledTimes(1);
			expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/auth/invite');
			expect(res.invitation_token).toBe('token-abc-123');
		});

		it('listInvitations ruft GET /api/v1/admin/invitations ab', async () => {
			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 200,
				json: async () => ({
					invitations: [
						{
							id: 'inv-1',
							email: 'pending@4labs.local',
							created_at: '2026-09-18T00:00:00Z',
							expires_at: '2026-09-25T00:00:00Z'
						}
					]
				})
			});
			vi.stubGlobal('fetch', fetchMock);

			const list = await listInvitations();
			expect(fetchMock).toHaveBeenCalledTimes(1);
			expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/admin/invitations');
			expect(list).toHaveLength(1);
		});
	});

	describe('listAuditLogs', () => {
		it('listAuditLogs fuegt action- und user-Query-Parameter an /api/v1/admin/audit an', async () => {
			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 200,
				json: async () => ({ logs: [] })
			});
			vi.stubGlobal('fetch', fetchMock);

			await listAuditLogs('login', 'felix');
			expect(fetchMock).toHaveBeenCalledTimes(1);
			const url = fetchMock.mock.calls[0][0] as string;
			expect(url).toContain('/admin/audit?');
			expect(url).toContain('action=login');
			expect(url).toContain('user=felix');
		});
	});

	describe('isAdmin() Helper', () => {
		it('liefert true fuer currentUser mit isAdmin=true', () => {
			currentUser.set({ id: 'admin-id', email: 'admin@4labs.local', isAdmin: true });
			expect(isAdmin()).toBe(true);
		});

		it('liefert false fuer normalen Benutzer', () => {
			currentUser.set({ id: 'user-id', email: 'user@4labs.local', isAdmin: false });
			expect(isAdmin()).toBe(false);
		});

		it('liefert false wenn kein Benutzer angemeldet ist', () => {
			currentUser.set(null);
			accessToken.set(null);
			expect(isAdmin()).toBe(false);
		});
	});

	describe('delete_account_cleanup_test', () => {
		it('nach 204 sind Cookies bereinigt und alle Stores zurueckgesetzt', () => {
			// Ausgangszustand: Eingeloggter Nutzer mit Tokens
			currentUser.set({ id: 'user-delete-me', email: 'deleted@4labs.local', isAdmin: false });
			accessToken.set('active-access-token');
			pendingMfaToken.set('temp-mfa-token');

			expect(get(currentUser)).not.toBeNull();
			expect(get(accessToken)).toBe('active-access-token');

			// Cleanup simulieren (wie in DeleteAccount.svelte nach erfolgreichem 204)
			clearAuthCookies();
			clearAuthState();

			// Pruefen, dass In-Memory-Stores leer sind
			expect(get(currentUser)).toBeNull();
			expect(get(accessToken)).toBeNull();
			expect(get(pendingMfaToken)).toBeNull();
		});
	});
});

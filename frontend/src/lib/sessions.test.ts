import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { listSessions, revokeSession, revokeOtherSessions } from './sessions';

describe('Sessions API Client (src/lib/sessions.ts)', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('listSessions ruft GET /api/v1/auth/sessions ab und liefert Sitzungsliste', async () => {
		const mockSessions = [
			{
				id: 'sess-1',
				ip: '127.0.0.1',
				user_agent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)',
				created_at: '2026-09-18T12:00:00Z',
				is_current: true
			},
			{
				id: 'sess-2',
				ip: '192.168.1.50',
				user_agent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0)',
				created_at: '2026-09-17T18:00:00Z',
				is_current: false
			}
		];

		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: async () => ({ sessions: mockSessions })
		});
		vi.stubGlobal('fetch', fetchMock);

		const result = await listSessions();

		expect(fetchMock).toHaveBeenCalledTimes(1);
		const [url, opts] = fetchMock.mock.calls[0];
		expect(url).toBe('/api/v1/auth/sessions');
		expect(result).toHaveLength(2);
		expect(result[0].id).toBe('sess-1');
		expect(result[0].is_current).toBe(true);
	});

	it('listSessions faengt Fehler ab und liefert leeres Array', async () => {
		const fetchMock = vi.fn().mockRejectedValue(new Error('Network error'));
		vi.stubGlobal('fetch', fetchMock);

		const result = await listSessions();
		expect(result).toEqual([]);
	});

	it('revokeSession sendet DELETE /api/v1/auth/sessions/:id', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			status: 204,
			json: async () => ({})
		});
		vi.stubGlobal('fetch', fetchMock);

		await revokeSession('session-uuid-123');

		expect(fetchMock).toHaveBeenCalledTimes(1);
		const [url, opts] = fetchMock.mock.calls[0];
		expect(url).toBe('/api/v1/auth/sessions/session-uuid-123');
		expect(opts?.method).toBe('DELETE');
	});

	it('revokeOtherSessions sendet DELETE /api/v1/auth/sessions', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			status: 204,
			json: async () => ({})
		});
		vi.stubGlobal('fetch', fetchMock);

		await revokeOtherSessions();

		expect(fetchMock).toHaveBeenCalledTimes(1);
		const [url, opts] = fetchMock.mock.calls[0];
		expect(url).toBe('/api/v1/auth/sessions');
		expect(opts?.method).toBe('DELETE');
	});
});

import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import { requestExport, getExportStatus, getExportDownloadUrl } from './exports';

describe('DSGVO Exports API Client (src/lib/exports.ts)', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	it('requestExport sendet POST /api/v1/users/me/export mit Passwort', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			status: 202,
			json: async () => ({ job_id: 'job-xyz-789', status: 'pending' })
		});
		vi.stubGlobal('fetch', fetchMock);

		const result = await requestExport('super-secret-pw');

		expect(fetchMock).toHaveBeenCalledTimes(1);
		const [url, opts] = fetchMock.mock.calls[0];
		expect(url).toBe('/api/v1/users/me/export');
		expect(opts?.method).toBe('POST');
		expect(JSON.parse(opts?.body as string)).toEqual({ password: 'super-secret-pw' });
		expect(result.job_id).toBe('job-xyz-789');
		expect(result.status).toBe('pending');
	});

	it('getExportStatus ruft GET /api/v1/users/me/export/:job_id ab', async () => {
		const fetchMock = vi.fn().mockResolvedValue({
			ok: true,
			status: 200,
			json: async () => ({
				job_id: 'job-xyz-789',
				status: 'completed',
				download_url: '/api/v1/users/me/export/job-xyz-789/download',
				expires_at: '2026-09-25T12:00:00Z'
			})
		});
		vi.stubGlobal('fetch', fetchMock);

		const status = await getExportStatus('job-xyz-789');

		expect(fetchMock).toHaveBeenCalledTimes(1);
		const [url] = fetchMock.mock.calls[0];
		expect(url).toBe('/api/v1/users/me/export/job-xyz-789');
		expect(status.status).toBe('completed');
		expect(status.download_url).toContain('/download');
	});

	it('getExportDownloadUrl erzeugt korrekten Pfad', () => {
		const url = getExportDownloadUrl('test-job-42');
		expect(url).toBe('/api/v1/users/me/export/test-job-42/download');
	});
});

import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import {
	listFiles,
	downloadFile,
	deleteFile,
	renameFile,
	createShare,
	formatBytes,
	formatDate
} from './files';
import { ApiError } from './api';

describe('Files-Client (src/lib/files.ts)', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	describe('Formatierungshelfer', () => {
		it('formatiert Dateigroessen korrekt', () => {
			expect(formatBytes(0)).toBe('0 B');
			expect(formatBytes(1024)).toBe('1 KB');
			expect(formatBytes(5 * 1024 * 1024)).toBe('5 MB');
			expect(formatBytes(2.5 * 1024 * 1024 * 1024)).toBe('2.5 GB');
		});

		it('formatiert ISO-Zeitstempel ins deutsche Format', () => {
			const formatted = formatDate('2026-09-18T14:30:00Z');
			expect(formatted).toMatch(/\d{2}\.\d{2}\.\d{4}/);
		});
	});

	describe('API-Aufrufe', () => {
		it('listFiles sendet korrekte Query-Parameter fuer Paginierung, Sortierung und Suche', async () => {
			const mockFiles = [
				{
					id: 'file-1',
					filename: 'bericht.pdf',
					size_bytes: 2048,
					mime_type: 'application/pdf',
					created_at: '2026-09-18T10:00:00Z'
				}
			];

			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 200,
				json: async () => ({
					files: mockFiles,
					total: 1,
					page: 2,
					limit: 25
				})
			});
			vi.stubGlobal('fetch', fetchMock);

			const result = await listFiles(2, 25, 'name', 'asc', 'bericht');

			expect(result.total).toBe(1);
			expect(result.files).toHaveLength(1);
			expect(fetchMock).toHaveBeenCalledTimes(1);

			const callUrl = fetchMock.mock.calls[0][0];
			expect(callUrl).toContain('/api/v1/files?');
			expect(callUrl).toContain('page=2');
			expect(callUrl).toContain('limit=25');
			expect(callUrl).toContain('sort=name');
			expect(callUrl).toContain('order=asc');
			expect(callUrl).toContain('filter=bericht');
		});

		it('downloadFile ruft /files/:id/download auf und liefert Presigned URL', async () => {
			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 200,
				json: async () => ({
					download_url: '/api/v1/uploads/tus/file-1/download?token=signed123',
					filename: 'dokument.pdf'
				})
			});
			vi.stubGlobal('fetch', fetchMock);

			const res = await downloadFile('file-1');

			expect(res.download_url).toBe('/api/v1/uploads/tus/file-1/download?token=signed123');
			expect(res.filename).toBe('dokument.pdf');
			expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/files/file-1/download');
		});

		it('deleteFile sendet DELETE-Anfrage an /files/:id', async () => {
			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 204
			});
			vi.stubGlobal('fetch', fetchMock);

			await deleteFile('file-delete-1');

			expect(fetchMock).toHaveBeenCalledTimes(1);
			expect(fetchMock.mock.calls[0][0]).toBe('/api/v1/files/file-delete-1');
			expect(fetchMock.mock.calls[0][1].method).toBe('DELETE');
		});

		it('renameFile sendet PATCH mit neuem Namen an /files/:id', async () => {
			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 200,
				json: async () => ({ id: 'file-1', filename: 'neuer_name.pdf' })
			});
			vi.stubGlobal('fetch', fetchMock);

			const res = await renameFile('file-1', 'neuer_name.pdf');

			expect(res.filename).toBe('neuer_name.pdf');
			const call = fetchMock.mock.calls[0];
			expect(call[0]).toBe('/api/v1/files/file-1');
			expect(call[1].method).toBe('PATCH');
			expect(JSON.parse(call[1].body)).toEqual({ filename: 'neuer_name.pdf' });
		});

		it('createShare sendet POST an /files/:id/share mit Optionen', async () => {
			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 201,
				json: async () => ({
					share_token: 'token-abc',
					share_url: '/share/token-abc',
					expires_at: '2026-09-25T12:00:00Z'
				})
			});
			vi.stubGlobal('fetch', fetchMock);

			const res = await createShare('file-1', { expires_days: 14, password: 'pw' });

			expect(res.share_token).toBe('token-abc');
			expect(res.share_url).toBe('/share/token-abc');
			const call = fetchMock.mock.calls[0];
			expect(call[0]).toBe('/api/v1/files/file-1/share');
			expect(call[1].method).toBe('POST');
			expect(JSON.parse(call[1].body)).toEqual({ expires_days: 14, password: 'pw' });
		});

		it('wirft ApiError bei Fehlern', async () => {
			const fetchMock = vi.fn().mockResolvedValue({
				ok: false,
				status: 404,
				json: async () => ({ error: 'Datei nicht gefunden' })
			});
			vi.stubGlobal('fetch', fetchMock);

			await expect(downloadFile('not-found-id')).rejects.toThrow(ApiError);
		});
	});
});

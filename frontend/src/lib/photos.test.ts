import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest';
import {
	getThumbnailUrl,
	getThumbnailBlob,
	revokeThumbnailBlob,
	listPhotos,
	listTags,
	createTag,
	assignTag,
	removeTag,
	getAlbumFiles,
	groupPhotosByDate,
	type PhotoItem
} from './photos';
import { accessToken } from './stores';

describe('Photos & Albums (src/lib/photos.ts)', () => {
	beforeEach(() => {
		vi.restoreAllMocks();
	});

	afterEach(() => {
		vi.unstubAllGlobals();
	});

	describe('Thumbnail-URL Generierung', () => {
		it('erzeugt korrekte Thumbnail-URL ueber den Go-Proxy', () => {
			const id = '123e4567-e89b-12d3-a456-426614174000';
			expect(getThumbnailUrl(id)).toBe(`/api/v1/files/${id}/thumbnail`);
		});
	});

	describe('Medien-Listen (listPhotos)', () => {
		it('setzt type=images Parameter bei Fotofilterung', async () => {
			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 200,
				json: async () => ({ files: [], total: 0, page: 1, limit: 50 })
			});
			vi.stubGlobal('fetch', fetchMock);

			await listPhotos('images', 1, 50);

			expect(fetchMock).toHaveBeenCalledTimes(1);
			const url = fetchMock.mock.calls[0][0] as string;
			expect(url).toContain('/files?');
			expect(url).toContain('type=images');
		});

		it('setzt type=videos Parameter bei Videofilterung', async () => {
			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 200,
				json: async () => ({ files: [], total: 0, page: 1, limit: 50 })
			});
			vi.stubGlobal('fetch', fetchMock);

			await listPhotos('videos', 1, 50);

			const url = fetchMock.mock.calls[0][0] as string;
			expect(url).toContain('type=videos');
		});
	});

	describe('Tag- und Alben-Verwaltung', () => {
		it('listTags ruft /tags ab', async () => {
			const mockTags = [
				{ id: 'tag-1', name: 'Urlaub 2025', color: '#3B82F6', file_count: 5, created_at: '2026-01-01' }
			];
			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 200,
				json: async () => ({ tags: mockTags })
			});
			vi.stubGlobal('fetch', fetchMock);

			const tags = await listTags();
			expect(tags).toEqual(mockTags);
			expect(fetchMock.mock.calls[0][0]).toContain('/tags');
		});

		it('createTag sendet POST /tags mit Name und Farbe', async () => {
			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 201,
				json: async () => ({ id: 'new-tag', name: 'Reisen', color: '#10B981' })
			});
			vi.stubGlobal('fetch', fetchMock);

			const created = await createTag('Reisen', '#10B981');
			expect(created.id).toBe('new-tag');
			const options = fetchMock.mock.calls[0][1] as RequestInit;
			expect(options.method).toBe('POST');
			expect(JSON.parse(options.body as string)).toEqual({ name: 'Reisen', color: '#10B981' });
		});

		it('assignTag verknuepft ein Tag mit einer Datei', async () => {
			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 200,
				json: async () => ({ status: 'ok', file_id: 'f-1', tag_id: 't-1' })
			});
			vi.stubGlobal('fetch', fetchMock);

			const res = await assignTag('f-1', { name: 'Natur' });
			expect(res.status).toBe('ok');
			expect(fetchMock.mock.calls[0][0]).toContain('/files/f-1/tags');
		});

		it('removeTag sendet DELETE an /files/:id/tags/:tag_id', async () => {
			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 200,
				json: async () => ({ status: 'ok' })
			});
			vi.stubGlobal('fetch', fetchMock);

			const res = await removeTag('f-1', 't-1');
			expect(res.status).toBe('ok');
			const options = fetchMock.mock.calls[0][1] as RequestInit;
			expect(options.method).toBe('DELETE');
			expect(fetchMock.mock.calls[0][0]).toContain('/files/f-1/tags/t-1');
		});

		it('getAlbumFiles ruft alle Dateien eines Tags ab', async () => {
			const mockAlbum = {
				tag_id: 't-1',
				name: 'Urlaub',
				files: []
			};
			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 200,
				json: async () => mockAlbum
			});
			vi.stubGlobal('fetch', fetchMock);

			const res = await getAlbumFiles('t-1');
			expect(res.name).toBe('Urlaub');
			expect(fetchMock.mock.calls[0][0]).toContain('/tags/t-1/files');
		});
	});

	describe('Datums-Gruppierung (groupPhotosByDate)', () => {
		it('gruppiert Fotos nach Datum', () => {
			const photos: PhotoItem[] = [
				{
					id: 'p-1',
					filename: 'foto1.jpg',
					size_bytes: 1000,
					mime_type: 'image/jpeg',
					taken_at: '2025-05-15T12:00:00Z',
					created_at: '2025-05-15T12:00:00Z'
				},
				{
					id: 'p-2',
					filename: 'foto2.jpg',
					size_bytes: 2000,
					mime_type: 'image/jpeg',
					taken_at: '2025-05-15T14:00:00Z',
					created_at: '2025-05-15T14:00:00Z'
				}
			];

			const groups = groupPhotosByDate(photos);
			expect(groups.length).toBe(1);
			expect(groups[0].photos.length).toBe(2);
		});
	});

	describe('Thumbnail-Blob Abruf & Caching (thumbnail_blob_test)', () => {
		it('holt Thumbnail per fetch mit Bearer-Token und erzeugt Blob-URL', async () => {
			const fakeBlob = new Blob(['fake-image-binary'], { type: 'image/jpeg' });
			const createObjectURLMock = vi.fn().mockReturnValue('blob:http://localhost/test-thumb-123');
			const revokeObjectURLMock = vi.fn();
			vi.stubGlobal('URL', {
				createObjectURL: createObjectURLMock,
				revokeObjectURL: revokeObjectURLMock
			});

			accessToken.set('test-jwt-token');

			const fetchMock = vi.fn().mockResolvedValue({
				ok: true,
				status: 200,
				blob: async () => fakeBlob
			});
			vi.stubGlobal('fetch', fetchMock);

			const fileId = 'photo-1234';
			const blobUrl = await getThumbnailBlob(fileId);

			expect(blobUrl).toBe('blob:http://localhost/test-thumb-123');
			expect(fetchMock).toHaveBeenCalledTimes(1);

			// Pruefen, ob Authorization-Header korrekt gesetzt ist
			const [callUrl, callInit] = fetchMock.mock.calls[0];
			expect(callUrl).toBe('/api/v1/files/photo-1234/thumbnail');
			expect(callInit.headers['Authorization']).toBe('Bearer test-jwt-token');
			expect(createObjectURLMock).toHaveBeenCalledWith(fakeBlob);

			// Zweiter Aufruf liefert das gecachte Ergebnis ohne neuen Fetch
			const cachedUrl = await getThumbnailBlob(fileId);
			expect(cachedUrl).toBe('blob:http://localhost/test-thumb-123');
			expect(fetchMock).toHaveBeenCalledTimes(1);

			// Revoke-Cleanup pruefen
			revokeThumbnailBlob(fileId);
			expect(revokeObjectURLMock).toHaveBeenCalledWith('blob:http://localhost/test-thumb-123');
		});
	});
});

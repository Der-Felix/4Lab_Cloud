import { api, apiBlob } from './api';
import type { FileItem, FileListResponse } from './files';

export type PhotoItem = FileItem;

export interface TagItem {
	id: string;
	name: string;
	color?: string;
	file_count?: number;
	created_at: string;
}

export interface AlbumResponse {
	tag_id: string;
	name: string;
	color?: string;
	files: PhotoItem[];
}

export interface PhotoGroup {
	title: string;
	photos: PhotoItem[];
}

// Erzeugt die Thumbnail-URL fuer ein Bild (Fallback/Referenz)
export function getThumbnailUrl(fileId: string): string {
	return `/api/v1/files/${fileId}/thumbnail`;
}

// Cache fuer erzeugte Thumbnail-Blob-URLs
const thumbnailBlobCache = new Map<string, string>();
const inFlightThumbnails = new Map<string, Promise<string>>();

// Holt Thumbnail via fetch() mit Bearer-Token, konvertiert zu Blob-URL und cached das Ergebnis
export async function getThumbnailBlob(fileId: string): Promise<string> {
	if (thumbnailBlobCache.has(fileId)) {
		return thumbnailBlobCache.get(fileId)!;
	}

	if (inFlightThumbnails.has(fileId)) {
		return inFlightThumbnails.get(fileId)!;
	}

	const fetchPromise = (async () => {
		try {
			const blob = await apiBlob(`/files/${fileId}/thumbnail`);
			const blobUrl = URL.createObjectURL(blob);
			thumbnailBlobCache.set(fileId, blobUrl);
			return blobUrl;
		} finally {
			inFlightThumbnails.delete(fileId);
		}
	})();

	inFlightThumbnails.set(fileId, fetchPromise);
	return fetchPromise;
}

// Gibt die Blob-URL fuer eine Datei frei (Memory-Cleanup bei Unmount)
export function revokeThumbnailBlob(fileId: string): void {
	const url = thumbnailBlobCache.get(fileId);
	if (url) {
		URL.revokeObjectURL(url);
		thumbnailBlobCache.delete(fileId);
	}
}

// Gibt alle gespeicherten Blob-URLs frei
export function revokeAllThumbnailBlobs(): void {
	for (const url of thumbnailBlobCache.values()) {
		URL.revokeObjectURL(url);
	}
	thumbnailBlobCache.clear();
}

// Ruft Fotos und Medien ab mit Filter nach Bild / Video
export async function listPhotos(
	type: 'all' | 'images' | 'videos' = 'all',
	page = 1,
	limit = 50,
	filter = ''
): Promise<FileListResponse> {
	const params = new URLSearchParams({
		page: String(page),
		limit: String(limit),
		sort: 'date',
		order: 'desc'
	});

	if (filter.trim()) {
		params.set('filter', filter.trim());
	}

	if (type === 'images') {
		params.set('type', 'images');
	} else if (type === 'videos') {
		params.set('type', 'videos');
	} else {
		params.set('type', 'media');
	}

	return api<FileListResponse>(`/files?${params.toString()}`);
}

// Ruft alle Tags (Alben) des aktuellen Benutzers ab
export async function listTags(): Promise<TagItem[]> {
	const res = await api<{ tags: TagItem[] }>('/tags');
	return res.tags || [];
}

// Erstellt ein neues Tag
export async function createTag(name: string, color?: string): Promise<TagItem> {
	return api<TagItem>('/tags', {
		method: 'POST',
		body: JSON.stringify({ name, color })
	});
}

// Weist einer Datei ein Tag zu
export async function assignTag(
	fileId: string,
	tag: { tag_id?: string; name?: string; color?: string }
): Promise<{ status: string; file_id: string; tag_id: string }> {
	return api<{ status: string; file_id: string; tag_id: string }>(`/files/${fileId}/tags`, {
		method: 'POST',
		body: JSON.stringify(tag)
	});
}

// Entfernt ein Tag von einer Datei
export async function removeTag(fileId: string, tagId: string): Promise<{ status: string }> {
	return api<{ status: string }>(`/files/${fileId}/tags/${tagId}`, {
		method: 'DELETE'
	});
}

// Ruft alle Dateien eines Albums/Tags ab
export async function getAlbumFiles(tagId: string): Promise<AlbumResponse> {
	return api<AlbumResponse>(`/tags/${tagId}/files`);
}

// Gruppiert Fotos chronologisch nach Aufnahmedatum oder Upload-Datum
export function groupPhotosByDate(photos: PhotoItem[]): PhotoGroup[] {
	const groupsMap = new Map<string, PhotoItem[]>();

	const now = new Date();
	const todayStr = now.toISOString().slice(0, 10);

	const yesterday = new Date(now);
	yesterday.setDate(yesterday.getDate() - 1);
	const yesterdayStr = yesterday.toISOString().slice(0, 10);

	for (const photo of photos) {
		const rawDate = photo.taken_at || photo.created_at;
		const dateObj = new Date(rawDate);
		const key = rawDate ? rawDate.slice(0, 10) : 'Unbekannt';

		let title = key;
		if (key === todayStr) {
			title = 'Heute';
		} else if (key === yesterdayStr) {
			title = 'Gestern';
		} else if (!isNaN(dateObj.getTime())) {
			title = new Intl.DateTimeFormat('de-DE', {
				month: 'long',
				year: 'numeric'
			}).format(dateObj);
		}

		const existing = groupsMap.get(title) || [];
		existing.push(photo);
		groupsMap.set(title, existing);
	}

	const groups: PhotoGroup[] = [];
	for (const [title, groupPhotos] of groupsMap.entries()) {
		groups.push({ title, photos: groupPhotos });
	}

	return groups;
}

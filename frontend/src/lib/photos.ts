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

// Gruppiert Fotos chronologisch nach Aufnahmedatum oder Upload-Datum (Alte Logik)
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

// Hierarchische Timeline-Struktur (Monat -> Tage -> Fotos)
export interface TimelineDay {
	date: string;       // "18. Sep"
	rawDate: string;    // "2026-09-18"
	photos: PhotoItem[];
}

export interface TimelineMonth {
	month: string;      // "September 2026"
	rawMonth: string;   // "2026-09"
	days: TimelineDay[];
}

// Gruppiert Fotos hierarchisch nach Monat und Tag
export function groupByTimeline(photos: PhotoItem[]): TimelineMonth[] {
	if (!photos || photos.length === 0) {
		return [];
	}

	// 1. Fotos chronologisch absteigend sortieren
	const sorted = [...photos].sort((a, b) => {
		const dateA = new Date(a.taken_at || a.created_at).getTime() || 0;
		const dateB = new Date(b.taken_at || b.created_at).getTime() || 0;
		return dateB - dateA;
	});

	// 2. Maps fuer geordnete Hierarchie (Key: YYYY-MM -> Key: YYYY-MM-DD -> PhotoItem[])
	const monthsMap = new Map<string, { label: string; daysMap: Map<string, { label: string; photos: PhotoItem[] }> }>();

	for (const photo of sorted) {
		const rawDate = photo.taken_at || photo.created_at;
		const dateObj = rawDate ? new Date(rawDate) : null;
		const isValid = dateObj && !isNaN(dateObj.getTime());

		const monthKey = isValid
			? `${dateObj.getFullYear()}-${String(dateObj.getMonth() + 1).padStart(2, '0')}`
			: 'unbekannt';

		const monthLabel = isValid
			? `${dateObj.toLocaleDateString('de-DE', { month: 'long' })} ${dateObj.getFullYear()}`
			: 'Unbekannt';

		const dayKey = isValid
			? dateObj.toISOString().slice(0, 10)
			: 'unbekannt';

		const dayLabel = isValid
			? `${dateObj.getDate()}. ${dateObj.toLocaleDateString('de-DE', { month: 'short' }).replace('.', '')}`
			: 'Unbekannt';

		if (!monthsMap.has(monthKey)) {
			monthsMap.set(monthKey, { label: monthLabel, daysMap: new Map() });
		}

		const monthEntry = monthsMap.get(monthKey)!;
		if (!monthEntry.daysMap.has(dayKey)) {
			monthEntry.daysMap.set(dayKey, { label: dayLabel, photos: [] });
		}

		monthEntry.daysMap.get(dayKey)!.photos.push(photo);
	}

	// 3. In hierarchische Arrays konvertieren
	const result: TimelineMonth[] = [];
	for (const [rawMonth, { label: monthLabel, daysMap }] of monthsMap.entries()) {
		const days: TimelineDay[] = [];
		for (const [rawDate, { label: dayLabel, photos: dayPhotos }] of daysMap.entries()) {
			days.push({
				date: dayLabel,
				rawDate,
				photos: dayPhotos
			});
		}
		result.push({
			month: monthLabel,
			rawMonth,
			days
		});
	}

	return result;
}

// Hilfsfunktionen zur EXIF-Formatierung
export function formatAperture(fNumber?: string | null): string {
	if (!fNumber) return '';
	const clean = fNumber.trim();
	if (clean.toLowerCase().startsWith('f/')) return clean;
	return `f/${clean}`;
}

export function formatExposure(exposure?: string | null): string {
	if (!exposure) return '';
	const clean = exposure.trim();
	if (clean.endsWith('s')) return clean;
	return `${clean}s`;
}

export function formatFocalLength(focal?: string | null): string {
	if (!focal) return '';
	const clean = focal.trim();
	if (clean.toLowerCase().endsWith('mm')) return clean;
	return `${clean} mm`;
}

export function formatISO(iso?: number | string | null): string {
	if (!iso) return '';
	const clean = String(iso).trim();
	if (clean.toUpperCase().startsWith('ISO')) return clean;
	return `ISO ${clean}`;
}

export function formatCoordinates(lat?: number | null, lon?: number | null): string {
	if (lat === undefined || lat === null || lon === undefined || lon === null) {
		return '';
	}
	const latCard = lat >= 0 ? 'N' : 'S';
	const lonCard = lon >= 0 ? 'E' : 'W';
	return `${Math.abs(lat).toFixed(4)}° ${latCard}, ${Math.abs(lon).toFixed(4)}° ${lonCard}`;
}


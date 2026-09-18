import { describe, it, expect } from 'vitest';
import {
	groupByTimeline,
	formatAperture,
	formatExposure,
	formatFocalLength,
	formatISO,
	formatCoordinates,
	type PhotoItem
} from './photos';

describe('photos timeline & formatters', () => {
	it('formats EXIF attributes properly', () => {
		expect(formatAperture('1.8')).toBe('f/1.8');
		expect(formatAperture('f/2.8')).toBe('f/2.8');
		expect(formatAperture(null)).toBe('');

		expect(formatExposure('1/250')).toBe('1/250s');
		expect(formatExposure('1/250s')).toBe('1/250s');
		expect(formatExposure(null)).toBe('');

		expect(formatFocalLength('50')).toBe('50 mm');
		expect(formatFocalLength('35 mm')).toBe('35 mm');
		expect(formatFocalLength(null)).toBe('');

		expect(formatISO(100)).toBe('ISO 100');
		expect(formatISO('ISO 400')).toBe('ISO 400');
		expect(formatISO(null)).toBe('');

		expect(formatCoordinates(47.3769, 8.5417)).toBe('47.3769° N, 8.5417° E');
		expect(formatCoordinates(-33.8688, 151.2093)).toBe('33.8688° S, 151.2093° E');
		expect(formatCoordinates(null, null)).toBe('');
	});

	it('groups photos into hierarchical months and days', () => {
		const mockPhotos: PhotoItem[] = [
			{
				id: 'p1',
				filename: 'mountain.jpg',
				size_bytes: 1024,
				mime_type: 'image/jpeg',
				created_at: '2026-09-18T10:00:00Z',
				taken_at: '2026-09-18T10:00:00Z'
			},
			{
				id: 'p2',
				filename: 'lake.jpg',
				size_bytes: 2048,
				mime_type: 'image/jpeg',
				created_at: '2026-09-18T12:00:00Z',
				taken_at: '2026-09-18T12:00:00Z'
			},
			{
				id: 'p3',
				filename: 'forest.jpg',
				size_bytes: 3072,
				mime_type: 'image/jpeg',
				created_at: '2026-08-05T15:00:00Z',
				taken_at: '2026-08-05T15:00:00Z'
			}
		];

		const timeline = groupByTimeline(mockPhotos);

		expect(timeline.length).toBe(2);

		// Monat 1: September 2026
		expect(timeline[0].month).toBe('September 2026');
		expect(timeline[0].days.length).toBe(1);
		expect(timeline[0].days[0].date).toBe('18. Sep');
		expect(timeline[0].days[0].photos.length).toBe(2);

		// Monat 2: August 2026
		expect(timeline[1].month).toBe('August 2026');
		expect(timeline[1].days.length).toBe(1);
		expect(timeline[1].days[0].date).toBe('5. Aug');
		expect(timeline[1].days[0].photos.length).toBe(1);
	});
});

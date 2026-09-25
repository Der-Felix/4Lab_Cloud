import { describe, it, expect, beforeAll, afterAll } from 'vitest';
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

	describe('Tag-Bucket ueber lokale Mitternacht hinweg (Regression)', () => {
		// Die dayKey-Berechnung in groupByTimeline muss auf LOKALEN Datumsteilen basieren, nicht auf UTC
		// (dayLabel und monthKey sind bereits lokal). Zwei Fotos, die in UTC am selben Tag liegen, aber
		// in der lokalen Zeitzone auf unterschiedliche Kalendertage fallen, duerfen NICHT im selben
		// Tag-Bucket landen.
		//
		// Zeitzonen-Sicherheit dieses Tests: wir pinnen process.env.TZ explizit auf 'Europe/Berlin', bevor
		// irgendein Date-Objekt erzeugt wird. Node liest process.env.TZ bei jedem Date-Zugriff neu aus
		// (siehe Verifikation), es gibt also kein Caching-Problem. Eine feste, von UTC verschiedene Zone
		// ist hier notwendig: laeuft der Test in einer Umgebung mit TZ=UTC (z. B. viele CI-Runner), faellt
		// der lokale Tag mit dem UTC-Tag zusammen und der Bug waere nicht reproduzierbar, egal wie die
		// Eingabe-Zeitstempel konstruiert werden. Innerhalb der gepinnten Zone werden die Eingaben zudem
		// bewusst so gewaehlt, dass sie lokale Mitternacht ueberschreiten (12:00 und 01:00 Uhr lokal an
		// aufeinanderfolgenden Kalendertagen), exakt der in der Fehlermeldung beschriebene Reproduktionsfall.
		let originalTz: string | undefined;

		beforeAll(() => {
			originalTz = process.env.TZ;
			process.env.TZ = 'Europe/Berlin';
		});

		afterAll(() => {
			process.env.TZ = originalTz;
		});

		it('ordnet Fotos anhand des LOKALEN Kalendertags ein, nicht des UTC-Tags', () => {
			const photos: PhotoItem[] = [
				{
					id: 'berlin-day18',
					filename: 'day18.jpg',
					size_bytes: 100,
					mime_type: 'image/jpeg',
					// 2026-09-18T10:00:00Z => Europe/Berlin (Sommerzeit, UTC+2) = 18.09.2026, 12:00 lokal
					created_at: '2026-09-18T10:00:00Z',
					taken_at: '2026-09-18T10:00:00Z'
				},
				{
					id: 'berlin-day19',
					filename: 'day19.jpg',
					size_bytes: 100,
					mime_type: 'image/jpeg',
					// 2026-09-18T23:00:00Z => Europe/Berlin (UTC+2) = 19.09.2026, 01:00 lokal
					// Beide Fotos liegen also am selben UTC-Kalendertag (18.09.), aber an ZWEI
					// verschiedenen lokalen Kalendertagen.
					created_at: '2026-09-18T23:00:00Z',
					taken_at: '2026-09-18T23:00:00Z'
				}
			];

			const timeline = groupByTimeline(photos);

			expect(timeline.length).toBe(1);
			expect(timeline[0].month).toBe('September 2026');

			// Der alte, UTC-basierte dayKey wuerde hier nur EINEN Tag-Bucket ("2026-09-18") erzeugen,
			// gelabelt mit dem Datum des zuerst eingefuegten (weil absteigend sortierten) Fotos: "19. Sep".
			// Korrekt sind ZWEI getrennte Tag-Buckets mit den jeweils lokal korrekten Labels.
			expect(timeline[0].days.length).toBe(2);

			const rawDates = timeline[0].days.map((d) => d.rawDate).sort();
			expect(rawDates).toEqual(['2026-09-18', '2026-09-19']);

			const day18 = timeline[0].days.find((d) => d.rawDate === '2026-09-18');
			const day19 = timeline[0].days.find((d) => d.rawDate === '2026-09-19');

			expect(day18?.date).toBe('18. Sep');
			expect(day18?.photos.map((p) => p.id)).toEqual(['berlin-day18']);

			expect(day19?.date).toBe('19. Sep');
			expect(day19?.photos.map((p) => p.id)).toEqual(['berlin-day19']);
		});
	});
});

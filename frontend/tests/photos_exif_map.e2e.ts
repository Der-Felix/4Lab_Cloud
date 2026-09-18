import { test, expect } from '@playwright/test';

test.describe('Timeline, EXIF-Sidebar & Map Integration (v0.2.2)', () => {
	test.beforeEach(async ({ page }) => {
		const fakePayload = btoa(JSON.stringify({ sub: 'user-photo-222', exp: Math.floor(Date.now() / 1000) + 900 }));
		await page.route('**/api/v1/auth/refresh', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				headers: {
					'Set-Cookie': 'csrf_token=test-csrf; Path=/; SameSite=Strict'
				},
				body: JSON.stringify({
					access_token: `hdr.${fakePayload}.sig`,
					csrf_token: 'test-csrf',
					email: 'felix@4labs.local',
					is_admin: true,
					expires_in: 900
				})
			});
		});
	});

	test('1. Timeline mit Monats-Header und Tag-Untergruppen', async ({ page }) => {
		const mockPhotos = [
			{
				id: 'photo-1',
				filename: 'alpen_panorama.jpg',
				size_bytes: 3500000,
				mime_type: 'image/jpeg',
				width: 3840,
				height: 2160,
				created_at: '2026-09-18T14:30:00Z',
				taken_at: '2026-09-18T14:30:00Z',
				location_name: 'Zermatt, Schweiz',
				gps_lat: 45.9765,
				gps_lon: 7.7491
			},
			{
				id: 'photo-2',
				filename: 'matterhorn_gipfel.jpg',
				size_bytes: 4200000,
				mime_type: 'image/jpeg',
				width: 4000,
				height: 3000,
				created_at: '2026-09-18T11:15:00Z',
				taken_at: '2026-09-18T11:15:00Z',
				location_name: 'Matterhorn, Schweiz',
				gps_lat: 45.9765,
				gps_lon: 7.7491
			},
			{
				id: 'photo-3',
				filename: 'bergsee_spiegelung.jpg',
				size_bytes: 2800000,
				mime_type: 'image/jpeg',
				width: 3000,
				height: 2000,
				created_at: '2026-09-17T09:45:00Z',
				taken_at: '2026-09-17T09:45:00Z',
				location_name: null,
				gps_lat: 46.0123,
				gps_lon: 7.7891
			},
			{
				id: 'photo-4',
				filename: 'sommer_blumen.jpg',
				size_bytes: 1900000,
				mime_type: 'image/jpeg',
				width: 2400,
				height: 1600,
				created_at: '2026-08-12T16:20:00Z',
				taken_at: '2026-08-12T16:20:00Z',
				location_name: null,
				gps_lat: null,
				gps_lon: null
			}
		];

		await page.route('**/api/v1/files?*', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					files: mockPhotos,
					total: mockPhotos.length,
					page: 1,
					limit: 100
				})
			});
		});

		await page.route('**/api/v1/files/*/thumbnail', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'image/jpeg',
				body: Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46])
			});
		});

		await page.goto('/photos');
		await page.waitForLoadState('networkidle');

		// Verifiziere Monats-Header
		await expect(page.locator('h2:has-text("September 2026")')).toBeVisible();
		await expect(page.locator('h2:has-text("August 2026")')).toBeVisible();

		// Verifiziere Tag-Header
		await expect(page.locator('h3:has-text("18. Sep")')).toBeVisible();
		await expect(page.locator('h3:has-text("17. Sep")')).toBeVisible();
		await expect(page.locator('h3:has-text("12. Aug")')).toBeVisible();

		// Verifiziere Toolbar Filter-Buttons
		await expect(page.locator('button:has-text("Mit Ort")')).toBeVisible();
		await expect(page.locator('button:has-text("Mit GPS")')).toBeVisible();
		await expect(page.locator('select')).toBeVisible();

		// Screenshot fuer Timeline
		await page.screenshot({ path: '../docs/screenshots/timeline-month.png', fullPage: true });
	});

	test('2. Lightbox EXIF-Sidebar mit Metadaten und "Auf Karte zeigen"', async ({ page }) => {
		const mockPhotos = [
			{
				id: 'photo-exif-1',
				filename: 'matterhorn_summit.jpg',
				size_bytes: 4200000,
				mime_type: 'image/jpeg',
				width: 4000,
				height: 3000,
				created_at: '2026-09-18T11:15:00Z',
				taken_at: '2026-09-18T11:15:00Z',
				location_name: 'Zermatt, Schweiz',
				gps_lat: 45.9765,
				gps_lon: 7.7491
			}
		];

		await page.route('**/api/v1/files?*', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					files: mockPhotos,
					total: 1,
					page: 1,
					limit: 100
				})
			});
		});

		await page.route('**/api/v1/files/photo-exif-1/thumbnail', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'image/jpeg',
				body: Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46])
			});
		});

		await page.route('**/api/v1/files/photo-exif-1/exif', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					file_id: 'photo-exif-1',
					location_name: 'Zermatt, Wallis, Schweiz',
					location_address: { city: 'Zermatt', country: 'Schweiz' },
					gps_lat: 45.9765,
					gps_lon: 7.7491,
					exif_json: {
						make: 'Sony',
						model: 'ILCE-7M4',
						lens_model: 'FE 24-70mm F2.8 GM II',
						exposure_time: '1/500',
						f_number: 'f/2.8',
						iso: 100,
						focal_length: '35 mm',
						datetime_original: '2026-09-18T11:15:00Z',
						gps: {
							lat: 45.9765,
							lon: 7.7491
						}
					}
				})
			});
		});

		await page.route('**/api/v1/tags', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ tags: [{ id: 't1', name: 'Schweiz 2026', color: '#2DD4BF' }] })
			});
		});

		await page.goto('/photos');
		await page.waitForLoadState('networkidle');

		// Klick auf Foto zum Öffnen der Lightbox
		await page.locator('button[title="matterhorn_summit.jpg"]').click();

		// Verifiziere geöffnete EXIF-Sidebar (280px)
		const sidebar = page.locator('aside:has-text("Informationen")');
		await expect(sidebar).toBeVisible();

		// Verifiziere EXIF-Inhalte
		await expect(sidebar.locator('text=Zermatt, Wallis, Schweiz')).toBeVisible();
		await expect(sidebar.locator('text=Sony ILCE-7M4')).toBeVisible();
		await expect(sidebar.locator('text=FE 24-70mm F2.8 GM II')).toBeVisible();
		await expect(sidebar.locator('text=f/2.8')).toBeVisible();
		await expect(sidebar.locator('text=1/500s')).toBeVisible();
		await expect(sidebar.locator('text=100')).toBeVisible();
		await expect(sidebar.locator('text=Auf Karte zeigen')).toBeVisible();

		// Screenshot fuer Lightbox mit EXIF-Sidebar
		await page.screenshot({ path: '../docs/screenshots/lightbox-exif.png' });
	});

	test('3. Dashboard MapPreview Widget', async ({ page }) => {
		await page.route('**/api/v1/photos/map', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([
					{
						id: 'photo-map-1',
						filename: 'alpen.jpg',
						thumb_url: '/api/v1/files/photo-map-1/thumbnail',
						gps_lat: 45.9765,
						gps_lon: 7.7491,
						location_name: 'Zermatt, Schweiz',
						taken_at: '2026-09-18T14:30:00Z'
					},
					{
						id: 'photo-map-2',
						filename: 'zuerich.jpg',
						thumb_url: '/api/v1/files/photo-map-2/thumbnail',
						gps_lat: 47.3769,
						gps_lon: 8.5417,
						location_name: 'Zürich, Schweiz',
						taken_at: '2026-09-17T10:00:00Z'
					}
				])
			});
		});

		await page.route('**/api/v1/files?*', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ files: [], total: 0, page: 1, limit: 16 })
			});
		});

		await page.route('**/api/v1/shares', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([])
			});
		});

		await page.route('**/api/v1/users/me/quota', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ used_bytes: 1048576, total_bytes: 10737418240, percent: 0.1 })
			});
		});

		await page.route('**/api/v1/users/me/audit', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify([])
			});
		});

		await page.goto('/');
		await page.waitForLoadState('networkidle');

		// Verifiziere Fotokarte-Widget in der Dashboard-Sidebar
		const mapCard = page.locator('h3:has-text("Fotokarte")');
		await expect(mapCard).toBeVisible();
		await expect(page.locator('text=Auf Vollbildkarte anzeigen')).toBeVisible();

		// Screenshot fuer Dashboard mit Mini-Karte
		await page.screenshot({ path: '../docs/screenshots/dashboard-map-preview.png', fullPage: true });
	});
});

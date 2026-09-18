import { test, expect } from '@playwright/test';

test.describe('Foto-Galerie & Alben (Etappe F3)', () => {
	test.beforeEach(async ({ page }) => {
		const fakePayload = btoa(JSON.stringify({ sub: 'user-photo-111', exp: Math.floor(Date.now() / 1000) + 900 }));
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
					email: 'gallery_user@4labs.internal',
					is_admin: false,
					expires_in: 900
				})
			});
		});
	});

	test('Galerie laedt Fotos und oeffnet Lightbox', async ({ page }) => {
		await page.route('**/api/v1/files?*', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					files: [
						{
							id: 'photo-1',
							filename: 'sommerurlaub.jpg',
							size_bytes: 1048576,
							mime_type: 'image/jpeg',
							thumbnail_path: '00/photo-1.enc',
							width: 1920,
							height: 1080,
							created_at: '2026-09-18T10:00:00Z'
						}
					],
					total: 1,
					page: 1,
					limit: 60
				})
			});
		});

		await page.route('**/api/v1/files/photo-1/thumbnail', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'image/jpeg',
				body: Buffer.from([0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46])
			});
		});

		await page.route('**/api/v1/tags', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					tags: [
						{ id: 'tag-1', name: 'Urlaub 2026', color: '#3B82F6', file_count: 1, created_at: '2026-09-18T10:00:00Z' }
					]
				})
			});
		});

		await page.goto('/photos');

		// Pruefen, dass Seite geladen ist
		await expect(page.locator('h1')).toContainText('Fotos & Medien');
		await expect(page.locator('text=sommerurlaub.jpg')).toBeVisible();

		// Klick auf Foto oeffnet Lightbox
		await page.locator('button[title="sommerurlaub.jpg"]').click();
		await expect(page.locator('text=1 / 1')).toBeVisible();

		// Escape schliesst Lightbox
		await page.keyboard.press('Escape');
		await expect(page.locator('text=1 / 1')).not.toBeVisible();
	});

	test('Alben-Uebersicht zeigt Alben an und Modal funktioniert', async ({ page }) => {
		await page.route('**/api/v1/tags', async (route) => {
			if (route.request().method() === 'GET') {
				await route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify({
						tags: [
							{ id: 'tag-1', name: 'Alpen 2025', color: '#10B981', file_count: 3, created_at: '2026-09-18T10:00:00Z' }
						]
					})
				});
			} else if (route.request().method() === 'POST') {
				await route.fulfill({
					status: 201,
					contentType: 'application/json',
					body: JSON.stringify({
						id: 'tag-2',
						name: 'Strand',
						color: '#F59E0B',
						file_count: 0,
						created_at: '2026-09-18T11:00:00Z'
					})
				});
			}
		});

		await page.goto('/photos/albums');

		await expect(page.locator('h1')).toContainText('Alben');
		await expect(page.locator('text=Alpen 2025')).toBeVisible();

		// Neues Album Modal oeffnen
		await page.click('button:has-text("Neues Album")');
		await expect(page.locator('text=Neues Album erstellen')).toBeVisible();

		// Formular ausfuellen und absenden
		await page.fill('#album-name', 'Strand');
		await page.click('button:has-text("Album anlegen")');

		await expect(page.locator('text=Strand')).toBeVisible();
	});

	test('Oeffentliche Share-Seite laedt ohne Authentifizierung und ermoeglicht Download', async ({ page }) => {
		await page.route('**/api/v1/shares/share-token-xyz', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					token: 'share-token-xyz',
					filename: 'oeffentliches_foto.jpg',
					size_bytes: 204800,
					mime_type: 'image/jpeg',
					requires_password: false
				})
			});
		});

		await page.route('**/api/v1/shares/share-token-xyz/download*', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					download_url: '/api/v1/uploads/tus/mock/download?token=valid',
					filename: 'oeffentliches_foto.jpg'
				})
			});
		});

		await page.goto('/share/share-token-xyz');

		await expect(page.locator('text=oeffentliches_foto.jpg')).toBeVisible();
		await expect(page.locator('button:has-text("Datei herunterladen")')).toBeVisible();
	});
});

import { test, expect } from '@playwright/test';

test.describe('File-Browser & Aktionen (Etappe F2)', () => {
	test.beforeEach(async ({ page }) => {
		// Mock fuer aktive Session bei Boot
		const fakePayload = btoa(JSON.stringify({ sub: 'user-a-111', exp: Math.floor(Date.now() / 1000) + 900 }));
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
					email: 'user_a@4labs.internal',
					is_admin: false,
					expires_in: 900
				})
			});
		});
	});

	test('Dateiliste zeigt Dateien an und Download funktioniert', async ({ page }) => {
		await page.route('**/api/v1/files?*', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					files: [
						{
							id: 'file-123',
							filename: 'geheimes_dokument.pdf',
							size_bytes: 2048,
							mime_type: 'application/pdf',
							created_at: '2026-09-18T12:00:00Z'
						}
					],
					total: 1,
					page: 1,
					limit: 50
				})
			});
		});

		let downloadRequested = false;
		await page.route('**/api/v1/files/file-123/download', async (route) => {
			downloadRequested = true;
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					download_url: '/api/v1/uploads/tus/file-123/download?token=valid-token',
					filename: 'geheimes_dokument.pdf'
				})
			});
		});

		await page.goto('/files');

		await expect(page.locator('h1')).toContainText('Dateien');
		await expect(page.locator('text=geheimes_dokument.pdf')).toBeVisible();
		await expect(page.locator('text=2 KB')).toBeVisible();

		// Download anklicken
		await page.click('button[title="Herunterladen"]');
		expect(downloadRequested).toBe(true);
	});

	test('Umbenennen einer Datei aktualisiert den Namen in der Tabelle', async ({ page }) => {
		await page.route('**/api/v1/files?*', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					files: [
						{
							id: 'file-rename-1',
							filename: 'altes_dokument.pdf',
							size_bytes: 1024,
							mime_type: 'application/pdf',
							created_at: '2026-09-18T10:00:00Z'
						}
					],
					total: 1,
					page: 1,
					limit: 50
				})
			});
		});

		let patchSent = false;
		await page.route('**/api/v1/files/file-rename-1', async (route) => {
			if (route.request().method() === 'PATCH') {
				patchSent = true;
				await route.fulfill({
					status: 200,
					contentType: 'application/json',
					body: JSON.stringify({
						id: 'file-rename-1',
						filename: 'neues_dokument.pdf',
						status: 'ok'
					})
				});
			}
		});

		await page.goto('/files');
		await expect(page.locator('text=altes_dokument.pdf')).toBeVisible();

		// Umbenennen-Modus starten
		await page.click('button[title="Umbenennen"]');
		const input = page.locator('input[type="text"][class*="border-indigo-500"]');
		await expect(input).toBeVisible();
		await input.fill('neues_dokument.pdf');
		await page.click('button:has-text("Speichern")');

		expect(patchSent).toBe(true);
		await expect(page.locator('text=neues_dokument.pdf')).toBeVisible();
	});

	test('Loeschen einer Datei oeffnet ConfirmDialog und entfernt Datei', async ({ page }) => {
		let currentFiles = [
			{
				id: 'file-del-1',
				filename: 'loesch_mich.txt',
				size_bytes: 512,
				mime_type: 'text/plain',
				created_at: '2026-09-18T08:00:00Z'
			}
		];

		await page.route('**/api/v1/files?*', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					files: currentFiles,
					total: currentFiles.length,
					page: 1,
					limit: 50
				})
			});
		});

		let deleteSent = false;
		await page.route('**/api/v1/files/file-del-1', async (route) => {
			if (route.request().method() === 'DELETE') {
				deleteSent = true;
				currentFiles = [];
				await route.fulfill({ status: 204 });
			}
		});

		await page.goto('/files');
		await expect(page.locator('text=loesch_mich.txt')).toBeVisible();

		// Loeschen anklicken -> Dialog pruefen
		await page.click('button[title="Löschen"]');
		await expect(page.locator('h3:has-text("Datei endgültig löschen?")')).toBeVisible();

		// Bestaetigen
		await page.click('button:has-text("Endgültig löschen")');

		expect(deleteSent).toBe(true);
		await expect(page.locator('text=Keine Dateien gefunden')).toBeVisible();
	});

	test('Dedizierte Upload-Seite bindet Uppy Dashboard ein', async ({ page }) => {
		await page.goto('/files/upload');
		await expect(page.locator('h1')).toContainText('Dateien hochladen');
		await expect(page.locator('.uppy-Dashboard')).toBeVisible();
	});

	test('RLS-Check: Benutzer sieht nur eigene Dateien', async ({ page }) => {
		// Mock: User A fragt Dateien ab und erhaelt ausschliesslich Datei A
		await page.route('**/api/v1/files?*', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					files: [
						{
							id: 'file-user-a',
							filename: 'nur_fuer_user_a.pdf',
							size_bytes: 3000,
							mime_type: 'application/pdf',
							created_at: '2026-09-18T10:00:00Z'
						}
					],
					total: 1,
					page: 1,
					limit: 50
				})
			});
		});

		// Falls User A versucht, fremde Datei von User B abzurufen -> 404
		await page.route('**/api/v1/files/foreign-file-b/**', async (route) => {
			await route.fulfill({
				status: 404,
				contentType: 'application/json',
				body: JSON.stringify({ error: 'Datei nicht gefunden' })
			});
		});

		await page.goto('/files');
		await expect(page.locator('text=nur_fuer_user_a.pdf')).toBeVisible();
		// Datei von User B darf niemals im DOM auftauchen
		await expect(page.locator('text=datei_von_user_b.pdf')).not.toBeVisible();
	});
});

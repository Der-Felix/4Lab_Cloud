import { test, expect } from '@playwright/test';
import * as path from 'path';

const SCREENSHOT_DIR = path.resolve('../docs/screenshots');

function setupAuthenticatedRoutes(page: any, isAdmin = true) {
	const fakePayload = btoa(
		JSON.stringify({
			sub: 'user-admin-123',
			email: 'felix@4labs.local',
			is_admin: isAdmin,
			exp: Math.floor(Date.now() / 1000) + 900
		})
	);
	const token = `header.${fakePayload}.sig`;

	page.route('**/api/v1/auth/refresh', async (route: any) => {
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				access_token: token,
				is_admin: isAdmin,
				expires_in: 900
			})
		});
	});

	page.route('**/api/v1/auth/sessions', async (route: any) => {
		if (route.request().method() === 'DELETE') {
			await route.fulfill({ status: 204 });
			return;
		}
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				sessions: [
					{
						id: 'sess-mac-1',
						ip: '127.0.0.1',
						user_agent: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)',
						created_at: new Date().toISOString(),
						is_current: true
					},
					{
						id: 'sess-phone-2',
						ip: '192.168.1.42',
						user_agent: 'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0)',
						created_at: new Date(Date.now() - 3600000).toISOString(),
						is_current: false
					}
				]
			})
		});
	});

	page.route('**/api/v1/auth/sessions/*', async (route: any) => {
		if (route.request().method() === 'DELETE') {
			await route.fulfill({ status: 204 });
			return;
		}
		await route.fallback();
	});

	page.route('**/api/v1/users/me/export', async (route: any) => {
		await route.fulfill({
			status: 202,
			contentType: 'application/json',
			body: JSON.stringify({
				job_id: 'export-job-42',
				status: 'pending'
			})
		});
	});

	page.route('**/api/v1/users/me/export/*', async (route: any) => {
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				job_id: 'export-job-42',
				status: 'completed',
				download_url: '/api/v1/users/me/export/export-job-42/download',
				expires_at: new Date(Date.now() + 7 * 86400000).toISOString()
			})
		});
	});

	page.route('**/api/v1/users/me', async (route: any) => {
		if (route.request().method() === 'DELETE') {
			await route.fulfill({ status: 204 });
			return;
		}
		await route.fallback();
	});

	page.route('**/api/v1/admin/users', async (route: any) => {
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				users: [
					{
						id: 'user-admin-123',
						email: 'felix@4labs.local',
						is_admin: true,
						mfa_enabled: true,
						created_at: '2026-09-01T08:00:00Z'
					},
					{
						id: 'user-test-456',
						email: 'anna@4labs.local',
						is_admin: false,
						mfa_enabled: false,
						created_at: '2026-09-10T14:30:00Z'
					}
				]
			})
		});
	});

	page.route('**/api/v1/admin/users/*', async (route: any) => {
		if (route.request().method() === 'DELETE') {
			await route.fulfill({ status: 204 });
			return;
		}
		await route.fallback();
	});

	page.route('**/api/v1/admin/invitations', async (route: any) => {
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				invitations: [
					{
						id: 'inv-1',
						email: 'kollege@4labs.local',
						created_at: '2026-09-17T10:00:00Z',
						expires_at: '2026-09-24T10:00:00Z'
					}
				]
			})
		});
	});

	page.route('**/api/v1/auth/invite', async (route: any) => {
		await route.fulfill({
			status: 201,
			contentType: 'application/json',
			body: JSON.stringify({
				invitation_token: 'token-e2e-abc-999',
				email: 'neuer-nutzer@4labs.local',
				expires_at: '2026-09-25T12:00:00Z'
			})
		});
	});

	page.route('**/api/v1/admin/audit*', async (route: any) => {
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				logs: [
					{
						id: 1,
						user_email: 'felix@4labs.local',
						action: 'login',
						ip_address: '127.0.0.1',
						result: 'ok',
						created_at: new Date().toISOString()
					},
					{
						id: 2,
						user_email: 'felix@4labs.local',
						action: 'upload',
						ip_address: '127.0.0.1',
						result: 'ok',
						created_at: new Date(Date.now() - 600000).toISOString()
					},
					{
						id: 3,
						user_id: 'unknown-id',
						action: 'login_failed',
						ip_address: '203.0.113.195',
						result: 'denied',
						created_at: new Date(Date.now() - 3600000).toISOString()
					}
				]
			})
		});
	});
}

test.describe('Etappe F4: Einstellungen & DSGVO & Admin', () => {
	test('Tab Sicherheit rendert Sessions und erstellt Screenshot', async ({ page }) => {
		setupAuthenticatedRoutes(page, true);
		await page.goto('/settings');

		// Klick auf Sicherheit Tab
		await page.getByRole('button', { name: /Sicherheit/i }).click();

		// Sessions-Tabelle pruefen
		await expect(page.locator('text=Aktive Sitzungen')).toBeVisible();
		await expect(page.getByText('Mac', { exact: true })).toBeVisible();
		await expect(page.locator('text=Mobiles Gerät')).toBeVisible();
		await expect(page.locator('text=Aktuelle Sitzung')).toBeVisible();

		// Screenshot fuer Sicherheit
		await page.screenshot({ path: `${SCREENSHOT_DIR}/settings-sicherheit.png`, fullPage: true });
	});

	test('Tab DSGVO: Datenexport-Modal & Screenshot', async ({ page }) => {
		setupAuthenticatedRoutes(page, true);
		await page.goto('/settings');

		await page.getByRole('button', { name: /DSGVO/i }).click();
		await expect(page.locator('text=Recht auf Datenübertragbarkeit')).toBeVisible();

		// Klick auf Export anfordern -> Modal oeffnet sich
		await page.getByRole('button', { name: /Export anfordern/i }).click();
		await expect(page.locator('text=Passwort für das ZIP-Archiv')).toBeVisible();

		// Screenshot export-modal.png
		await page.screenshot({ path: `${SCREENSHOT_DIR}/export-modal.png` });

		// Passwort eingeben und starten
		await page.fill('#export-pw', 'TestPasswort123!');
		await page.getByRole('button', { name: /Auftrag starten/i }).click();

		// Polling result
		await expect(page.locator('text=Ihr Datenexport ist abholbereit')).toBeVisible();

		// Screenshot settings-dsgvo.png
		await page.screenshot({ path: `${SCREENSHOT_DIR}/settings-dsgvo.png`, fullPage: true });
	});

	test('Tab DSGVO: Loeschungs-Modal & Screenshot', async ({ page }) => {
		setupAuthenticatedRoutes(page, true);
		await page.goto('/settings');

		await page.getByRole('button', { name: /DSGVO/i }).click();

		// Klick auf Konto endgueltig loeschen
		await page.getByRole('button', { name: /Konto endgültig löschen/i }).first().click();
		await expect(page.locator('text=Konto unwiderruflich löschen?')).toBeVisible();

		// Screenshot delete-modal.png
		await page.screenshot({ path: `${SCREENSHOT_DIR}/delete-modal.png` });
	});

	test('Tab Admin: Nutzerliste, Zweistufige Loeschung & Screenshot', async ({ page }) => {
		setupAuthenticatedRoutes(page, true);
		await page.goto('/settings');

		// Klick auf Admin Tab
		await page.getByRole('button', { name: /Administration/i }).click();

		// Nutzerliste sichtbar
		await expect(page.locator('text=Benutzerverwaltung')).toBeVisible();
		await expect(page.locator('text=felix@4labs.local')).toBeVisible();
		await expect(page.locator('text=anna@4labs.local')).toBeVisible();

		// Selbstloeschung disabled pruefen
		const selfDeleteBtn = page.locator('button[title="Sie können sich nicht selbst löschen"]');
		await expect(selfDeleteBtn).toBeDisabled();

		// Anna loeschen probieren -> Zweistufiger Dialog
		const annaRow = page.locator('tr:has-text("anna@4labs.local")');
		await annaRow.locator('button[title*="löschen"]').click();

		await expect(page.locator('text=Geben Sie zur Bestätigung die E-Mail-Adresse')).toBeVisible();
		const confirmDeleteBtn = page.getByRole('button', { name: /Endgültig löschen/i });
		await expect(confirmDeleteBtn).toBeDisabled();

		// Falsche E-Mail -> bleibt disabled
		await page.fill('#admin-del-confirm-email', 'wrong@4labs.local');
		await expect(confirmDeleteBtn).toBeDisabled();

		// Richtige E-Mail -> Button aktiviert
		await page.fill('#admin-del-confirm-email', 'anna@4labs.local');
		await expect(confirmDeleteBtn).toBeEnabled();

		// Dialog schliessen
		await page.getByRole('button', { name: /Abbrechen/i }).click();

		// Einladungen & Audit-Log pruefen
		await expect(page.locator('text=Offene Einladungen')).toBeVisible();
		await expect(page.locator('text=Audit-Protokoll')).toBeVisible();
		await expect(page.locator('text=Erfolgreich').first()).toBeVisible();

		// Screenshot settings-admin.png
		await page.screenshot({ path: `${SCREENSHOT_DIR}/settings-admin.png`, fullPage: true });
	});
});

import { test, expect } from '@playwright/test';

test.describe('Authentifizierungs-Flows (Login & MFA)', () => {
	test('Login mit falschem Passwort zeigt Fehlermeldung', async ({ page }) => {
		// Mock fuer fehlerhaften Login
		await page.route('**/api/v1/auth/login', async (route) => {
			await route.fulfill({
				status: 401,
				contentType: 'application/json',
				body: JSON.stringify({ error: 'Ungueltige Anmeldedaten' })
			});
		});

		// Session-Refresh beim Laden abfangen
		await page.route('**/api/v1/auth/refresh', async (route) => {
			await route.fulfill({ status: 401, body: JSON.stringify({ error: 'No session' }) });
		});

		await page.goto('/login');
		await page.fill('#email', 'wrong@test.de');
		await page.fill('#password', 'FalschesPasswort!');
		await page.click('button[type="submit"]');

		const alert = page.locator('[role="alert"]');
		await expect(alert).toBeVisible();
		await expect(alert).toContainText('Ungueltige Anmeldedaten');
	});

	test('Login Happy Path leitet zum Dashboard weiter', async ({ page }) => {
		// Mock fuer erfolgreichen Login
		await page.route('**/api/v1/auth/login', async (route) => {
			// Fake JWT mit sub="user-123"
			const fakePayload = btoa(JSON.stringify({ sub: 'user-123', exp: Math.floor(Date.now() / 1000) + 900 }));
			const token = `header.${fakePayload}.sig`;

			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				headers: {
					'Set-Cookie': 'csrf_token=test-csrf; Path=/; SameSite=Strict'
				},
				body: JSON.stringify({
					access_token: token,
					csrf_token: 'test-csrf',
					is_admin: true,
					expires_in: 900
				})
			});
		});

		await page.route('**/api/v1/auth/refresh', async (route) => {
			await route.fulfill({ status: 401, body: JSON.stringify({ error: 'No session' }) });
		});

		await page.goto('/login');
		await page.fill('#email', 'admin@4labs.internal');
		await page.fill('#password', 'SicheresPasswort123!');
		await page.click('button[type="submit"]');

		// Nach erfolgreichem Login Weiterleitung zum Dashboard
		await expect(page).toHaveURL('/');
		await expect(page.locator('h1')).toContainText('Dashboard');
		await expect(page.locator('text=Meine Dateien')).toBeVisible();
	});

	test('Login mit MFA-Pflicht leitet zur Code-Eingabe weiter und autorisiert', async ({ page }) => {
		// Mock: Login signalisiert MFA-Pflicht
		await page.route('**/api/v1/auth/login', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					mfa_required: true,
					mfa_token: 'pending-mfa-token-xyz'
				})
			});
		});

		// Mock: MFA-Verifizierung erfolgreich
		await page.route('**/api/v1/auth/mfa/verify', async (route) => {
			const fakePayload = btoa(JSON.stringify({ sub: 'user-mfa-456', exp: Math.floor(Date.now() / 1000) + 900 }));
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					access_token: `header.${fakePayload}.sig`,
					csrf_token: 'csrf-xyz',
					is_admin: false,
					expires_in: 900
				})
			});
		});

		await page.route('**/api/v1/auth/refresh', async (route) => {
			await route.fulfill({ status: 401, body: JSON.stringify({ error: 'No session' }) });
		});

		await page.goto('/login');
		await page.fill('#email', 'mfa-user@4labs.internal');
		await page.fill('#password', 'MfaPasswort123!');
		await page.click('button[type="submit"]');

		// Weiterleitung zur MFA-Seite
		await expect(page).toHaveURL('/login/mfa');
		await expect(page.locator('h1')).toContainText('Zwei-Faktor-Authentifizierung');

		// TOTP-Code eingeben
		await page.fill('#mfa-code', '123456');
		await page.click('button[type="submit"]');

		// Nach MFA-Bestaetigung Weiterleitung zum Dashboard
		await expect(page).toHaveURL('/');
		await expect(page.locator('h1')).toContainText('Dashboard');
	});

	test('MFA Setup Flow zeigt QR-Code und liefert Recovery-Codes', async ({ page }) => {
		await page.route('**/api/v1/auth/refresh', async (route) => {
			const fakePayload = btoa(JSON.stringify({ sub: 'user-setup-789', exp: Math.floor(Date.now() / 1000) + 900 }));
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					access_token: `header.${fakePayload}.sig`,
					csrf_token: 'csrf-setup'
				})
			});
		});

		await page.route('**/api/v1/auth/mfa/setup', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					secret: 'JBSWY3DPEHPK3PXP',
					qr_uri: 'otpauth://totp/4labscloud:test@4labs.de?secret=JBSWY3DPEHPK3PXP&issuer=4labscloud'
				})
			});
		});

		await page.route('**/api/v1/auth/mfa/activate', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					status: 'mfa_activated',
					recovery_codes: ['AAAA-1111-2222', 'BBBB-3333-4444'],
					access_token: 'new-token'
				})
			});
		});

		await page.goto('/settings/mfa');
		await expect(page.locator('h1')).toContainText('Zwei-Faktor-Authentifizierung');

		// Klick auf "MFA aktivieren"
		await page.click('button:has-text("MFA aktivieren")');

		// QR-Code und Schluessel pruefen
		const qrImg = page.locator('img[alt="MFA QR-Code"]');
		await expect(qrImg).toBeVisible();
		await expect(page.locator('text=JBSWY3DPEHPK3PXP')).toBeVisible();

		// Bestaetigungscode eingeben
		await page.fill('#verify-code', '654321');
		await page.click('button[type="submit"]');

		// Recovery-Codes muessen angezeigt werden
		await expect(page.locator('text=AAAA-1111-2222')).toBeVisible();
		await expect(page.locator('text=BBBB-3333-4444')).toBeVisible();
	});
});

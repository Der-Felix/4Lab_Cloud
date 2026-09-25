import { test, expect } from '@playwright/test';
import * as path from 'path';

const SCREENSHOT_DIR = path.resolve('../docs/screenshots');

const MOCK_FILES = [
	{
		id: 'file-1',
		filename: 'Jahresbericht_2026.pdf',
		size_bytes: 2450000,
		mime_type: 'application/pdf',
		created_at: new Date(Date.now() - 1000 * 60 * 45).toISOString() // vor 45 Min
	},
	{
		id: 'file-2',
		filename: 'Budget_Planung.xlsx',
		size_bytes: 850000,
		mime_type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
		created_at: new Date(Date.now() - 1000 * 60 * 60 * 3).toISOString() // vor 3 Std
	},
	{
		id: 'file-3',
		filename: 'Projekt_Whitepaper.docx',
		size_bytes: 1250000,
		mime_type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
		created_at: new Date(Date.now() - 1000 * 60 * 60 * 24).toISOString() // vor 1 Tag
	},
	{
		id: 'file-4',
		filename: 'Team_Foto_Alpen.jpg',
		size_bytes: 4500000,
		mime_type: 'image/jpeg',
		created_at: new Date(Date.now() - 1000 * 60 * 60 * 36).toISOString() // vor 1 Tag
	},
	{
		id: 'file-5',
		filename: 'Architektur_Diagramm.png',
		size_bytes: 820000,
		mime_type: 'image/png',
		created_at: new Date(Date.now() - 1000 * 60 * 60 * 48).toISOString() // vor 2 Tagen
	},
	{
		id: 'file-6',
		filename: 'Audio_Meeting_Notiz.mp3',
		size_bytes: 15200000,
		mime_type: 'audio/mpeg',
		created_at: new Date(Date.now() - 1000 * 60 * 60 * 72).toISOString() // vor 3 Tagen
	}
];

const MOCK_SHARES = [
	{
		id: 'share-1',
		file_id: 'file-1',
		filename: 'Jahresbericht_2026.pdf',
		size_bytes: 2450000,
		token: 'tok-public-123',
		share_url: 'http://localhost:5173/share/tok-public-123',
		has_password: false,
		expires_at: new Date(Date.now() + 1000 * 60 * 60 * 24 * 7).toISOString(),
		created_at: new Date(Date.now() - 1000 * 60 * 60 * 2).toISOString()
	},
	{
		id: 'share-2',
		file_id: 'file-2',
		filename: 'Budget_Planung.xlsx',
		size_bytes: 850000,
		token: 'tok-secret-456',
		share_url: 'http://localhost:5173/share/tok-secret-456',
		has_password: true,
		expires_at: new Date(Date.now() + 1000 * 60 * 60 * 24 * 3).toISOString(),
		created_at: new Date(Date.now() - 1000 * 60 * 60 * 5).toISOString()
	}
];

const MOCK_AUDIT_LOGS = [
	{
		id: 1,
		action: 'file_upload',
		result: 'success',
		created_at: new Date(Date.now() - 1000 * 60 * 12).toISOString()
	},
	{
		id: 2,
		action: 'share_create',
		result: 'success',
		created_at: new Date(Date.now() - 1000 * 60 * 45).toISOString()
	},
	{
		id: 3,
		action: 'auth_login',
		result: 'success',
		created_at: new Date(Date.now() - 1000 * 60 * 60 * 2).toISOString()
	}
];

const MOCK_QUOTA = {
	used_bytes: 2500000000, // ca. 2.3 GB
	total_bytes: 53687091200, // 50 GB
	percent: 4.7
};

function setupAuthenticatedRoutes(page: any) {
	const fakePayload = btoa(
		JSON.stringify({
			sub: 'user-admin-123',
			email: 'felix@4labs.local',
			is_admin: true,
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
				is_admin: true,
				expires_in: 900
			})
		});
	});

	page.route('**/api/v1/files?*', async (route: any) => {
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				files: MOCK_FILES,
				total: MOCK_FILES.length,
				page: 1,
				limit: 16
			})
		});
	});

	page.route('**/api/v1/shares', async (route: any) => {
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				shares: MOCK_SHARES
			})
		});
	});

	page.route('**/api/v1/users/me/quota', async (route: any) => {
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify(MOCK_QUOTA)
		});
	});

	page.route('**/api/v1/audit/me', async (route: any) => {
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				logs: MOCK_AUDIT_LOGS
			})
		});
	});

	page.route('**/api/v1/photos*', async (route: any) => {
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify({
				photos: [],
				total: 2,
				page: 1,
				limit: 1
			})
		});
	});

	// Mock fuer Thumbnails (Aesthetisches Vektorbild mit Bergen und Sonne)
	page.route('**/api/v1/files/*/thumbnail', async (route: any) => {
		const svg = `
		<svg xmlns="http://www.w3.org/2000/svg" width="400" height="400" viewBox="0 0 400 400">
		  <defs>
		    <linearGradient id="bg" x1="0%" y1="0%" x2="100%" y2="100%">
		      <stop offset="0%" stop-color="#141416" />
		      <stop offset="100%" stop-color="#1F2430" />
		    </linearGradient>
		    <linearGradient id="m1" x1="0%" y1="0%" x2="0%" y2="100%">
		      <stop offset="0%" stop-color="#2DD4BF" stop-opacity="0.8" />
		      <stop offset="100%" stop-color="#1E6FD9" stop-opacity="0.4" />
		    </linearGradient>
		  </defs>
		  <rect width="400" height="400" fill="url(#bg)"/>
		  <circle cx="310" cy="110" r="32" fill="#3BC1E8" opacity="0.4" />
		  <polygon points="40,340 180,150 290,340" fill="url(#m1)" />
		  <polygon points="180,340 280,190 380,340" fill="url(#m1)" opacity="0.75" />
		</svg>`.trim();

		await route.fulfill({
			status: 200,
			contentType: 'image/svg+xml',
			body: svg
		});
	});
}

test.describe('Dashboard Layout & Farboptimierung V2 E2E', () => {
	test('Dashboard rendert 280px Hero-Banner, Bild-Overlay, 2-Spalten-Raster, Tiefe, Quota & Aktivitaet', async ({ page }) => {
		setupAuthenticatedRoutes(page);
		await page.setViewportSize({ width: 1440, height: 900 });

		await page.goto('/');

		// 1. Hero-Banner pruefen (280px Hoehe, hero-default.jpg, dunkles Overlay, 2 Buttons, Great Vibes Slogan)
		const heroImg = page.locator('img[src="/assets/hero-default.jpg"]');
		await expect(heroImg).toBeVisible();
		await expect(page.locator('.bg-black\\/40')).toBeVisible();

		await expect(page.locator('h1')).toContainText('Schön, dich wieder zu sehen, Felix!');
		await expect(page.locator('text=Deine Dateien, Fotos und Projekte an einem sicheren Ort.')).toBeVisible();
		await expect(page.getByRole('link', { name: /Dateien hochladen/i })).toBeVisible();
		await expect(page.getByRole('link', { name: /Fotos öffnen/i })).toBeVisible();
		// "Ordner erstellen" darf NICHT existieren
		await expect(page.locator('button:has-text("Ordner erstellen")')).not.toBeVisible();
		await expect(page.locator('text=Deine Daten. Deine Freiheit.')).toBeVisible();

		// 2. Kachel-Raster & Section-Header mit 2x16px Akzentbalken pruefen
		await expect(page.locator('h2')).toContainText('Zuletzt verwendet');
		await expect(page.getByRole('button', { name: 'Alle' })).toBeVisible();
		await expect(page.getByRole('button', { name: 'Dateien' })).toBeVisible();
		await expect(page.getByRole('button', { name: 'Bilder' })).toBeVisible();
		await expect(page.getByRole('button', { name: 'Dokumente' })).toBeVisible();

		// Alle Kacheln im Grid pruefen mit kategorie-spezifischen Badges & neutralem Namen
		const pdfCard = page.locator('[role="group"]:has-text("Jahresbericht_2026.pdf")');
		await expect(pdfCard).toBeVisible();
		await expect(pdfCard.getByText('PDF', { exact: true })).toBeVisible();

		const xlsxCard = page.locator('[role="group"]:has-text("Budget_Planung.xlsx")');
		await expect(xlsxCard).toBeVisible();
		await expect(xlsxCard.getByText('Tabelle', { exact: true })).toBeVisible();

		const imgCard = page.locator('[role="group"]:has-text("Team_Foto_Alpen.jpg")');
		await expect(imgCard).toBeVisible();
		await expect(imgCard.getByText('Bild', { exact: true })).toBeVisible();

		// Filter-Tab "Bilder" testen
		await page.getByRole('button', { name: 'Bilder' }).click();
		await expect(page.locator('[role="group"]:has-text("Team_Foto_Alpen.jpg")')).toBeVisible();
		await expect(page.locator('[role="group"]:has-text("Architektur_Diagramm.png")')).toBeVisible();
		await expect(page.locator('[role="group"]:has-text("Jahresbericht_2026.pdf")')).not.toBeVisible();

		// Filter-Tab "Dokumente" testen
		await page.getByRole('button', { name: 'Dokumente' }).click();
		await expect(page.locator('[role="group"]:has-text("Jahresbericht_2026.pdf")')).toBeVisible();
		await expect(page.locator('[role="group"]:has-text("Budget_Planung.xlsx")')).toBeVisible();
		await expect(page.locator('[role="group"]:has-text("Team_Foto_Alpen.jpg")')).not.toBeVisible();

		// Zurueck auf "Alle"
		await page.getByRole('button', { name: 'Alle' }).click();
		await expect(page.locator('[role="group"]:has-text("Jahresbericht_2026.pdf")')).toBeVisible();

		// 3. Rechte Sidebar pruefen: Ihre Freigaben, Speicherplatz & Aktivitaet
		await expect(page.locator('text=Ihre Freigaben')).toBeVisible();
		await expect(page.locator('text=Öffentlich').first()).toBeVisible();
		await expect(page.locator('text=Passwortgeschützt')).toBeVisible();

		// Speicherplatz als Kennzahl-Kachel: Beleg und Kontingent stehen jetzt in einer Zeile
		await expect(page.locator('text=Speicherplatz')).toBeVisible();
		await expect(page.locator('text=4.7%')).toBeVisible();
		await expect(page.locator('text=2.3 GB von 50 GB')).toBeVisible();

		// Karte "Letzte Aktivitaet" pruefen. Nicht auf "Aktivität" allein pruefen -
		// das trifft auch die gleichnamige Kennzahl-Kachel und verletzt den Strict Mode.
		await expect(page.locator('text=Letzte Aktivität')).toBeVisible();
		await expect(page.locator('text=Datei hochgeladen')).toBeVisible();
		await expect(page.locator('text=Freigabe erstellt')).toBeVisible();
		await expect(page.locator('text=Erfolgreich angemeldet')).toBeVisible();

		// 4. Die Compliance-Badges ("BSI TR-02102-2 & DSGVO konform", "AES-256-GCM at rest")
		// wurden bewusst aus dem Dashboard entfernt - hier gibt es nichts mehr zu pruefen.

		// 5. Dark Mode Screenshot erstellen (dashboard-v2-dark.png und dashboard-dark.png)
		await page.waitForTimeout(500);
		await page.screenshot({ path: `${SCREENSHOT_DIR}/dashboard-v2-dark.png`, fullPage: true });
		await page.screenshot({ path: `${SCREENSHOT_DIR}/dashboard-dark.png`, fullPage: true });

		// 6. Umschalten auf Light Mode und Light Mode Screenshot erstellen
		const themeToggle = page.getByRole('button', { name: /Hell|Dunkel/i });
		await themeToggle.click();
		await page.waitForTimeout(500);

		// Pruefen, dass dark-Klasse vom html-Element entfernt wurde
		const isDark = await page.evaluate(() => document.documentElement.classList.contains('dark'));
		expect(isDark).toBe(false);

		await page.screenshot({ path: `${SCREENSHOT_DIR}/dashboard-v2-light.png`, fullPage: true });
		await page.screenshot({ path: `${SCREENSHOT_DIR}/dashboard-light.png`, fullPage: true });
	});

	test('Empty-State wird korrekt gerendert mit Linear-Illustrationen', async ({ page }) => {
		setupAuthenticatedRoutes(page);
		// Ueberschreibe Files-Route mit leerer Liste
		await page.route('**/api/v1/files?*', async (route) => {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({
					files: [],
					total: 0,
					page: 1,
					limit: 16
				})
			});
		});

		await page.setViewportSize({ width: 1440, height: 900 });

		// 1. Dashboard Empty-State pruefen
		await page.goto('/');
		const emptyIllustration = page.locator('img[src="/assets/illustrations/empty-files.svg"]');
		await expect(emptyIllustration).toBeVisible();
		await expect(page.locator('text=Keine Dateien vorhanden')).toBeVisible();
		await expect(page.locator('text=Laden Sie Ihre erste Datei hoch.')).toBeVisible();

		// 2. /files Empty-State pruefen und Screenshot erstellen
		await page.goto('/files');
		const filesEmptyIllustration = page.locator('img[src="/assets/illustrations/empty-files.svg"]');
		await expect(filesEmptyIllustration).toBeVisible();
		await expect(page.locator('text=Keine Dateien vorhanden')).toBeVisible();

		await page.waitForTimeout(500);
		await page.screenshot({ path: `${SCREENSHOT_DIR}/empty-files.png`, fullPage: true });
	});

	test('Sidebar Expanded und Collapsed Zustaende mit Tabler Icons', async ({ page }) => {
		setupAuthenticatedRoutes(page);
		await page.setViewportSize({ width: 1440, height: 900 });

		await page.goto('/');

		// 1. Expanded Zustand pruefen
		const aside = page.locator('aside');
		await expect(aside).toHaveClass(/w-60/);

		// Gruppen-Header pruefen
		await expect(page.locator('aside').getByText('DASHBOARD', { exact: true })).toBeVisible();
		await expect(page.locator('aside').getByText('DATEIEN', { exact: true })).toBeVisible();
		await expect(page.locator('aside').getByText('SYSTEM', { exact: true })).toBeVisible();

		// Aktives Dashboard Item: filled icon, voller primary hintergrund, border-l-[3px] border-primary-light
		const activeItem = page.locator('aside a[href="/"]:has-text("Dashboard")');
		await expect(activeItem).toBeVisible();
		await expect(activeItem).toHaveClass(/bg-primary/);
		await expect(activeItem).toHaveClass(/text-white/);
		await expect(activeItem).toHaveClass(/border-primary-light/);

		// Tabler Icon im DOM vorhanden (Filled Variante)
		await expect(activeItem.locator('svg.tabler-icon')).toBeVisible();

		// User-Bereich unten: 40px Avatar, 2-Zeiler, Logout-Icon
		await expect(page.locator('aside').getByText('felix@4labs.local')).toBeVisible();
		await expect(page.locator('aside').getByText('Administrator')).toBeVisible();
		await expect(page.locator('aside button[title="Abmelden"] svg.tabler-icon')).toBeVisible();

		// Screenshot: docs/screenshots/sidebar-expanded.png
		await page.waitForTimeout(400);
		await page.screenshot({ path: `${SCREENSHOT_DIR}/sidebar-expanded.png`, fullPage: true });

		// 2. Collapse Toggle klicken
		const collapseBtn = page.locator('button[title="Sidebar einklappen"]');
		await expect(collapseBtn).toBeVisible();
		await collapseBtn.click();
		await page.waitForTimeout(400);

		// 3. Collapsed Zustand pruefen
		await expect(aside).toHaveClass(/w-16/);
		// Gruppen-Header duerfen im Collapsed-Zustand nicht sichtbar sein
		await expect(page.locator('aside').getByText('DASHBOARD', { exact: true })).not.toBeVisible();

		// Expand Button muss sichtbar sein
		const expandBtn = page.locator('button[title="Sidebar ausklappen"]');
		await expect(expandBtn).toBeVisible();

		// Screenshot: docs/screenshots/sidebar-collapsed.png
		await page.screenshot({ path: `${SCREENSHOT_DIR}/sidebar-collapsed.png`, fullPage: true });
	});

	test('Sidebar auf schmalem Viewport: eingeklappt, ohne irrefuehrenden Schalter', async ({ page }) => {
		setupAuthenticatedRoutes(page);
		// 390px entspricht einem gaengigen Telefon-Viewport
		await page.setViewportSize({ width: 390, height: 844 });
		await page.goto('/');
		await page.waitForTimeout(400);

		const aside = page.locator('aside');
		await expect(aside).toBeVisible();

		// Unterhalb md erzwingt isNarrow den eingeklappten Zustand (64px statt 240px).
		const width = await aside.evaluate((el) => Math.round(el.getBoundingClientRect().width));
		expect(width).toBeLessThanOrEqual(80);

		// Der Schalter darf hier nicht erscheinen: er koennte den versprochenen Zustand
		// nicht herstellen und wuerde nur die Desktop-Praeferenz veraendern.
		await expect(page.locator('button[title="Sidebar ausklappen"]')).toBeHidden();
		await expect(page.locator('button[title="Sidebar einklappen"]')).toBeHidden();

		// Der Kopfbereich darf nicht ueberlaufen. Die rechte Kante des <header> selbst
		// genuegt als Pruefung NICHT: die Hauptspalte setzt overflow-x-hidden, dadurch
		// bleibt der Header im Viewport, waehrend seine Kinder darueber hinausragen und
		// unerreichbar abgeschnitten werden - genau der urspruengliche Fehler.
		const overflow = await page.locator('header').evaluate((el) => {
			const vw = document.documentElement.clientWidth;
			let maxRight = 0;
			let worst = '';
			for (const child of el.querySelectorAll('*')) {
				const cs = getComputedStyle(child);
				if (cs.display === 'none' || cs.visibility === 'hidden') continue;
				if (!child.getClientRects().length) continue;
				const right = child.getBoundingClientRect().right;
				if (right > maxRight) {
					maxRight = right;
					worst = child.tagName.toLowerCase() + '.' + String(child.className).slice(0, 40);
				}
			}
			return { vw, maxRight: Math.round(maxRight), worst };
		});
		expect(
			overflow.maxRight,
			`Element ragt aus dem Viewport: ${overflow.worst} (rechte Kante ${overflow.maxRight} > ${overflow.vw})`
		).toBeLessThanOrEqual(overflow.vw);
	});
});

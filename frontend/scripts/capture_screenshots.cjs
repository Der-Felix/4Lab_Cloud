const { chromium } = require('@playwright/test');
const { spawn } = require('child_process');
const path = require('path');
const fs = require('fs');

async function main() {
	const outDir = path.resolve(__dirname, '../../docs/screenshots');
	if (!fs.existsSync(outDir)) {
		fs.mkdirSync(outDir, { recursive: true });
	}

	console.log('Starting preview server on port 4188...');
	const preview = spawn('npx', ['vite', 'preview', '--port', '4188'], {
		cwd: path.resolve(__dirname, '..'),
		stdio: 'pipe'
	});

	await new Promise((resolve) => {
		preview.stdout.on('data', (d) => {
			const str = d.toString();
			if (str.includes('4188') || str.includes('Local:')) {
				resolve();
			}
		});
		setTimeout(resolve, 3000);
	});

	console.log('Launching browser...');
	const browser = await chromium.launch({ headless: true });

	const fakePayload = Buffer.from(
		JSON.stringify({
			sub: 'usr-felix',
			email: 'felix@4labs.local',
			is_admin: true,
			exp: Math.floor(Date.now() / 1000) + 3600
		})
	).toString('base64');
	const fakeJwt = `eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.${fakePayload}.fakesignature`;

	const fakeRefreshResponse = {
		access_token: fakeJwt,
		email: 'felix@4labs.local',
		is_admin: true,
		expires_in: 3600
	};

	const mockFiles = [
		{ id: 'f-1', filename: 'Jahresbericht_2026.pdf', size_bytes: 4280000, mime_type: 'application/pdf', created_at: '2026-09-18T14:20:00Z' },
		{ id: 'f-2', filename: 'Labor_Messreihe_Q3.csv', size_bytes: 890000, mime_type: 'text/csv', created_at: '2026-09-18T11:05:00Z' },
		{ id: 'f-3', filename: 'Architektur_Konzept_v2.docx', size_bytes: 1450000, mime_type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', created_at: '2026-09-17T16:40:00Z' },
		{ id: 'f-4', filename: 'Campus_Drohnenflug_4K.mp4', size_bytes: 384000000, mime_type: 'video/mp4', created_at: '2026-09-16T09:15:00Z' },
		{ id: 'f-5', filename: 'Backup_DB_Dump_20260915.tar.gz', size_bytes: 52400000, mime_type: 'application/gzip', created_at: '2026-09-15T22:00:00Z' },
		{ id: 'f-6', filename: 'Design_System_Tokens.json', size_bytes: 45000, mime_type: 'application/json', created_at: '2026-09-15T12:00:00Z' },
		{ id: 'f-7', filename: 'Sicherheitsaudit_BSI_2026.pdf', size_bytes: 8700000, mime_type: 'application/pdf', created_at: '2026-09-14T08:30:00Z' }
	];

	const mockPhotos = [
		{ id: 'p-1', filename: 'Campus_Hauptgebaeude.jpg', size_bytes: 3400000, mime_type: 'image/jpeg', created_at: '2026-09-18T12:30:00Z' },
		{ id: 'p-2', filename: 'Labor_Mikroskop_Aufnahme.png', size_bytes: 4800000, mime_type: 'image/png', created_at: '2026-09-18T10:15:00Z' },
		{ id: 'p-3', filename: 'Server_Rack_Cluster.jpg', size_bytes: 2900000, mime_type: 'image/jpeg', created_at: '2026-09-17T15:00:00Z' },
		{ id: 'p-4', filename: 'Team_Meeting_Q3.jpg', size_bytes: 5100000, mime_type: 'image/jpeg', created_at: '2026-09-17T11:45:00Z' }
	];

	const mockShares = [
		{ id: 's-1', filename: 'Jahresbericht_2026.pdf', share_url: '/share/tok-1', size_bytes: 4280000, has_password: true, expires_at: '2026-10-01T00:00:00Z', created_at: '2026-09-18T14:25:00Z' },
		{ id: 's-2', filename: 'Labor_Messreihe_Q3.csv', share_url: '/share/tok-2', size_bytes: 890000, has_password: false, expires_at: null, created_at: '2026-09-17T17:00:00Z' }
	];

	const sampleSvg = Buffer.from(
		`<svg xmlns="http://www.w3.org/2000/svg" width="400" height="400" viewBox="0 0 400 400"><rect width="400" height="400" fill="#1E6FD9"/><rect x="40" y="40" width="320" height="320" rx="16" fill="#0F3A6E"/><circle cx="200" cy="180" r="60" fill="#3BC1E8"/><path d="M100 320 L200 240 L300 320 Z" fill="#2DD4BF"/></svg>`
	).toString('base64');

	// 1. Screenshot: login-dark.png
	{
		console.log('Capturing login-dark.png...');
		const loginContext = await browser.newContext({
			viewport: { width: 1440, height: 900 },
			deviceScaleFactor: 2
		});
		const page = await loginContext.newPage();
		await page.route('**/api/v1/auth/refresh', async (route) => {
			await route.fulfill({ status: 401, body: JSON.stringify({ error: 'unauthorized' }) });
		});
		await page.goto('http://localhost:4188/login');
		await page.waitForSelector('input#email');
		await page.evaluate(() => {
			document.documentElement.classList.add('dark');
			localStorage.setItem('theme', 'dark');
		});
		await page.waitForTimeout(300);
		console.log('Login URL:', page.url(), 'Title:', await page.title());
		await page.screenshot({ path: path.join(outDir, 'login-dark.png') });
		await loginContext.close();
	}

	// Authentifizierter Kontext fuer Dashboard, Files, Photos
	const authContext = await browser.newContext({
		viewport: { width: 1440, height: 900 },
		deviceScaleFactor: 2
	});

	// Setze CSRF Cookie
	await authContext.addCookies([
		{ name: 'csrf_token', value: 'fake-csrf-token', domain: 'localhost', path: '/' }
	]);

	const authPage = await authContext.newPage();

	await authPage.route('**/api/v1/auth/refresh', async (route) => {
		await route.fulfill({
			status: 200,
			contentType: 'application/json',
			body: JSON.stringify(fakeRefreshResponse)
		});
	});

	const realJpegPath = path.join(__dirname, '../../scratch/test_real.jpeg');
	const sampleJpegBuffer = fs.existsSync(realJpegPath) ? fs.readFileSync(realJpegPath) : Buffer.from(sampleSvg, 'base64');

	await authPage.route('**/api/v1/files?*', async (route) => {
		const url = route.request().url();
		if (url.includes('type=media') || url.includes('type=images') || url.includes('type=videos')) {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ files: mockPhotos, total: mockPhotos.length, page: 1, limit: 50 })
			});
		} else {
			await route.fulfill({
				status: 200,
				contentType: 'application/json',
				body: JSON.stringify({ files: mockFiles, total: mockFiles.length, page: 1, limit: 50 })
			});
		}
	});

	await authPage.route('**/api/v1/files/*/thumbnail', async (route) => {
		await route.fulfill({
			status: 200,
			contentType: fs.existsSync(realJpegPath) ? 'image/jpeg' : 'image/svg+xml',
			body: sampleJpegBuffer
		});
	});

	await authPage.route('**/api/v1/shares', async (route) => {
		await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(mockShares) });
	});

	// 2. Screenshot: dashboard-dark.png
	console.log('Capturing dashboard-dark.png...');
	await authPage.goto('http://localhost:4188/');
	await authPage.waitForSelector('h1:has-text("Willkommen zurück")');
	await authPage.evaluate(() => {
		document.documentElement.classList.add('dark');
		localStorage.setItem('theme', 'dark');
	});
	await authPage.waitForTimeout(500);
	console.log('Dashboard Dark URL:', authPage.url(), 'Title:', await authPage.title());
	await authPage.screenshot({ path: path.join(outDir, 'dashboard-dark.png') });

	// 3. Screenshot: dashboard-light.png
	console.log('Capturing dashboard-light.png...');
	await authPage.evaluate(() => {
		document.documentElement.classList.remove('dark');
		localStorage.setItem('theme', 'light');
	});
	await authPage.waitForTimeout(500);
	console.log('Dashboard Light URL:', authPage.url(), 'Title:', await authPage.title());
	await authPage.screenshot({ path: path.join(outDir, 'dashboard-light.png') });

	// Reset to dark
	await authPage.evaluate(() => {
		document.documentElement.classList.add('dark');
		localStorage.setItem('theme', 'dark');
	});

	// 4. Screenshot: files-dark.png
	console.log('Capturing files-dark.png...');
	await authPage.goto('http://localhost:4188/files');
	await authPage.waitForSelector('h1:has-text("Dateien")');
	await authPage.waitForTimeout(500);
	console.log('Files URL:', authPage.url(), 'Title:', await authPage.title());
	await authPage.screenshot({ path: path.join(outDir, 'files-dark.png') });

	// 5. Screenshot: photos-dark.png
	console.log('Capturing photos-dark.png...');
	await authPage.goto('http://localhost:4188/photos');
	await authPage.waitForSelector('h1:has-text("Fotos & Medien")');
	await authPage.waitForTimeout(500);
	console.log('Photos URL:', authPage.url(), 'Title:', await authPage.title());
	await authPage.screenshot({ path: path.join(outDir, 'photos-dark.png') });

	console.log('All screenshots captured successfully!');
	await browser.close();
	preview.kill();
	process.exit(0);
}

main().catch((err) => {
	console.error('Screenshot capture failed:', err);
	process.exit(1);
});

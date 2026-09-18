---
name: sveltekit-uppy
description: SvelteKit + Uppy Konventionen fuer 4labscloud Frontend. Bei jedem Frontend-Code laden.
---

# SvelteKit + Uppy

## Setup
- SvelteKit 2.x mit Adapter-Node (kein Static - braucht SSR fuer Auth-Cookies)
- TypeScript strict
- Styling: Tailwind CSS v4
- Upload: Uppy 4.x mit @uppy/tus

## Struktur
src/
├── routes/
│   ├── +layout.svelte        # Nav, Auth-Check
│   ├── +page.svelte          # Dashboard
│   ├── login/+page.svelte
│   ├── files/+page.svelte
│   ├── photos/+page.svelte
│   └── shares/+page.svelte
├── lib/
│   ├── api.ts                # Fetch-Wrapper zu Go
│   ├── auth.ts               # Session-Handling
│   ├── uploader.ts           # Uppy-Setup
│   └── components/
│       ├── FileList.svelte
│       ├── PhotoGrid.svelte
│       └── ShareDialog.svelte
└── app.html

## API-Client
const API_BASE = '/api/v1';
export async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(API_BASE + path, {
    ...init,
    credentials: 'include',
    headers: { 'Content-Type': 'application/json', ...init?.headers },
  });
  if (!res.ok) throw new ApiError(res.status, await res.text());
  return res.json();
}

## Uppy-Setup (Tus)
import Uppy from '@uppy/core';
import Tus from '@uppy/tus';
import Dashboard from '@uppy/dashboard';

const uppy = new Uppy({ restrictions: { maxFileSize: 5_000_000_000 } })
  .use(Dashboard, { inline: true, target: '#uploader' })
  .use(Tus, {
    endpoint: '/api/v1/uploads/tus',
    chunkSize: 5 * 1024 * 1024,
    retryDelays: [0, 1000, 3000, 5000],
  });

## Wichtig
- Keine externen CDNs (DSGVO) - alle Fonts/Icons lokal
- Kein Google Analytics, kein Sentry, kein Tracking
- Cookies: HttpOnly, Secure, SameSite=Strict
- Session via httpOnly Cookie, nicht localStorage
- Bilder NICHT direkt vom Storage laden - immer ueber Go (Auth-Pruefung!)

## Verboten
- localStorage fuer Tokens
- Inline-Skripte (CSP!)
- Externe Bilder/Fonts

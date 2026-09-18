import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

// Healthcheck-Endpoint: liefert immer 200, unabhaengig von Auth.
export const GET: RequestHandler = async () => {
    return json({ status: 'ok' }, { status: 200 });
};

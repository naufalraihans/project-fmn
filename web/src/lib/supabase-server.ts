// Klien Supabase sisi server (dipakai di +layout.server.ts).
// Membaca sesi dari cookie, bukan localStorage, sehingga penjagaan halaman
// bekerja pada render pertama, bukan setelah hydration.
import { createServerClient } from '@supabase/ssr';
import type { RequestEvent } from '@sveltejs/kit';
import { env as envPriv } from '$env/dynamic/private';
import { env as envPub } from '$env/dynamic/public';

export function klienServer(event: RequestEvent) {
	const url = envPub.PUBLIC_SUPABASE_URL;
	const anon = envPriv.SUPABASE_ANON_KEY || envPub.PUBLIC_SUPABASE_ANON_KEY;
	if (!url || !anon) throw new Error('Supabase belum dikonfigurasi di server.');
	return createServerClient(url, anon, {
		cookies: {
			getAll: () => event.cookies.getAll(),
			setAll: (cookies) => {
				cookies.forEach(({ name, value, options }) => event.cookies.set(name, value, { ...options, path: '/' }));
			}
		}
	});
}

export function apiBase(): string {
	return (envPub.PUBLIC_API_BASE ?? 'http://localhost:8080').replace(/\/$/, '');
}

// Klien Supabase sisi browser. Satu instans dipakai ulang di seluruh halaman.
// URL dibaca dari PUBLIC_SUPABASE_URL; anon key dari PUBLIC_SUPABASE_ANON_KEY
// (keduanya env PUBLIK - aman diekspos ke browser).
import { createBrowserClient } from '@supabase/ssr';
import { env } from '$env/dynamic/public';
import type { SupabaseClient } from '@supabase/supabase-js';

let client: SupabaseClient | null = null;

export function supabase(): SupabaseClient {
	if (client) return client;
	const url = env.PUBLIC_SUPABASE_URL;
	const anon = env.PUBLIC_SUPABASE_ANON_KEY;
	if (!url || !anon) {
		throw new Error(
			'Supabase belum dikonfigurasi. Isi PUBLIC_SUPABASE_URL dan PUBLIC_SUPABASE_ANON_KEY.'
		);
	}
	client = createBrowserClient(url, anon);
	return client;
}

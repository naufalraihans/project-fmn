// Status sesi yang dipakai seluruh halaman internal.
// Disimpan di Svelte store agar header/sidebar bereaksi saat login/logout.
import { writable } from 'svelte/store';

export type Peran = 'superadmin' | 'admin' | 'user';

export interface Sesi {
	userId: string;
	email: string;
	peran: Peran;
	wajibGantiPassword: boolean;
}

export const sesi = writable<Sesi | null>(null);

const KUNCI = 'fmn-sesi';

/** Muat sesi dari localStorage saat aplikasi dibuka. */
export function muatSesi(): Sesi | null {
	try {
		const raw = localStorage.getItem(KUNCI);
		if (!raw) return null;
		const s = JSON.parse(raw) as Sesi;
		if (!s.userId || !s.email || !s.peran) return null;
		sesi.set(s);
		return s;
	} catch {
		return null;
	}
}

export function simpanSesi(s: Sesi) {
	localStorage.setItem(KUNCI, JSON.stringify(s));
	sesi.set(s);
}

export function hapusSesi() {
	localStorage.removeItem(KUNCI);
	sesi.set(null);
}

/** Ambil access token Supabase saat ini (untuk header Bearer ke backend Go). */
export async function ambilToken(): Promise<string | null> {
	const { supabase } = await import('./supabase');
	try {
		const { data } = await supabase().auth.getSession();
		return data.session?.access_token ?? null;
	} catch {
		return null;
	}
}

import type { BlokKonten, Layanan, PeralatanPublik, Portofolio } from '$lib/compro';
import { api } from '$lib/api';

// Muat konten compro dari backend. Bila backend belum hidup (mis. saat build
// statis atau konten DB masih kosong), kembalikan struktur kosong supaya
// halaman TETAP tayang dengan konten bawaan dummy - bukan halaman galat.
export interface Compro {
	hero: BlokKonten | undefined;
	tentang: BlokKonten | undefined;
	kontak: BlokKonten | undefined;
	layanan: Layanan[];
	peralatan: PeralatanPublik[];
	portofolio: Portofolio[];
}

const KOSONG: Compro = {
	hero: undefined,
	tentang: undefined,
	kontak: undefined,
	layanan: [],
	peralatan: [],
	portofolio: []
};

export async function muatCompro(base?: string): Promise<Compro> {
	try {
		const [konten, porto] = await Promise.all([
			api<Record<string, BlokKonten | Layanan[] | PeralatanPublik[]>>('/api/public/content', { base }),
			api<Portofolio[]>('/api/public/portfolio?per_page=8', { base }).catch(() => ({ data: [] }))
		]);
		const d = konten.data ?? {};
		const ambil = <T,>(k: string): T | undefined => (d[k] as T | undefined) ?? undefined;
		return {
			hero: ambil<BlokKonten>('hero'),
			tentang: ambil<BlokKonten>('tentang') ?? ambil<BlokKonten>('about'),
			kontak: ambil<BlokKonten>('kontak') ?? ambil<BlokKonten>('contact'),
			layanan: (ambil<Layanan[]>('layanan') ?? ambil<Layanan[]>('services') ?? []) as Layanan[],
			peralatan: (ambil<PeralatanPublik[]>('peralatan') ?? ambil<PeralatanPublik[]>('equipment') ?? []) as PeralatanPublik[],
			portofolio: porto.data ?? []
		};
	} catch {
		return KOSONG;
	}
}

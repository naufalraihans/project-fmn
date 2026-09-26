// Tipe data compro. Bentuknya mengikuti docs/arch/openapi.yaml.
export interface BlokKonten {
	key: string;
	isi: Record<string, unknown>;
	published: boolean;
	updated_at: string;
}

export interface Layanan {
	kode: string;
	nama: string;
	deskripsi: string;
	cakupan: string[];
	urutan: number;
}

export interface PeralatanPublik {
	kategori: string;
	nama: string;
	keterangan: string;
}

export interface Portofolio {
	id: string;
	nama_event: string;
	lokasi: string;
	tahun: number;
	peran: string;
	deskripsi: string;
	foto_url: string | null;
}

/** Ambil string dari blok konten dengan aman (isi bebas, jadi bisa apa saja). */
export function teks(blok: BlokKonten | undefined, kunci: string, cadangan = ''): string {
	if (!blok) return cadangan;
	const v = blok.isi?.[kunci];
	return typeof v === 'string' ? v : cadangan;
}

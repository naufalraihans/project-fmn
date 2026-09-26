// Label Bahasa Indonesia untuk nilai enum backend dan tanggal.
/** 2026-09-26 -> "Sabtu, 26 September 2026". Dipakai di seluruh tampilan. */
export function tanggalPanjang(iso: string): string {
	try {
		const d = new Date(iso.length === 10 ? iso + 'T00:00:00+07:00' : iso);
		return new Intl.DateTimeFormat('id-ID', {
			weekday: 'long',
			day: 'numeric',
			month: 'long',
			year: 'numeric',
			timeZone: 'Asia/Jakarta'
		}).format(d);
	} catch {
		return iso;
	}
}

/** 2026-09-26T17:20:00+07:00 -> "17:20". Jam saja, tanpa tanggal. */
export function jam(iso: string | null | undefined): string {
	if (!iso) return '-';
	try {
		return new Intl.DateTimeFormat('id-ID', {
			hour: '2-digit',
			minute: '2-digit',
			timeZone: 'Asia/Jakarta'
		}).format(new Date(iso));
	} catch {
		return iso;
	}
}

/** 23865000 -> "Rp23.865.000". */
export function rupiah(n: number | null | undefined): string {
	if (n === null || n === undefined) return '-';
	return 'Rp' + new Intl.NumberFormat('id-ID').format(n);
}

const labelStatus: Record<string, string> = {
	hadir: 'Hadir',
	tidak_lengkap: 'Tidak lengkap',
	alpha: 'Alpha',
	draft: 'Draf',
	terkirim: 'Terkirim',
	dibayar: 'Lunas',
	batal: 'Batal',
	aktif: 'Aktif',
	nonaktif: 'Nonaktif',
	tersedia: 'Tersedia',
	dipakai: 'Dipakai',
	perawatan: 'Perawatan',
	baik: 'Baik',
	rusak_ringan: 'Rusak ringan',
	rusak_berat: 'Rusak berat'
};

const labelKategori: Record<string, string> = {
	rigging_stage: 'Rigging & Stage',
	sound: 'Sound System',
	lighting: 'Lighting',
	led_screen: 'LED / Videotron',
	genset: 'Genset',
	lain_lain: 'Lainnya'
};

const labelPeran: Record<string, string> = {
	superadmin: 'Superadmin',
	admin: 'Admin',
	user: 'Kru'
};

/** Nilai enum backend -> label Indonesia. Nilai asing dikembalikan apa adanya. */
export function label(v: string | null | undefined): string {
	if (!v) return '-';
	return labelStatus[v] ?? labelKategori[v] ?? labelPeran[v] ?? v;
}

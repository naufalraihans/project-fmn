// Penjaga halaman internal. Dipakai di setiap +page.ts di bawah /app:
// bila belum login, tendang ke /masuk; bila peran tidak cocok, tendang ke
// dashboard perannya sendiri. Aturan peran HALAMAN boleh longgar karena aturan
// sebenarnya ditegakkan backend (RBAC fail-closed); ini hanya soal kenyamanan,
// bukan keamanan.
import { redirect } from '@sveltejs/kit';
import type { Peran } from '$lib/sesi';

export function jaga(peranIzin: Peran[], sesi: { peran: Peran } | null, tujuan: string) {
	if (!sesi) throw redirect(303, `/masuk?lanjut=${encodeURIComponent(tujuan)}`);
	if (!peranIzin.includes(sesi.peran)) {
		const rumah = sesi.peran === 'superadmin' ? '/app/keuangan' : sesi.peran === 'admin' ? '/app/absensi' : '/app/absen';
		throw redirect(303, rumah);
	}
}

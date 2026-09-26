// Realtime Supabase untuk dashboard internal (ADR-001).
// Kanal privat fmn:ops (operasional) dan fmn:finance (superadmin).
// RLS di database yang menentukan siapa boleh mendengar; klien hanya memasang
// telinga. Gagal berlangganan tidak boleh merusak halaman.
import { supabase } from './supabase';

export type Kanal = 'fmn:ops' | 'fmn:finance';

export function langganan(kanal: Kanal, saatAda: (pesan: unknown) => void): () => void {
	let berhenti = false;
	let channel: { unsubscribe: () => void } | null = null;
	(async () => {
		try {
			const ch = supabase()
				.channel(kanal, { config: { private: true } })
				.on('broadcast', { event: '*' }, (payload) => {
					if (!berhenti) saatAda(payload);
				});
			await ch.subscribe();
			if (berhenti) {
				ch.unsubscribe();
				return;
			}
			channel = ch;
		} catch {
			// diam: halaman tetap bekerja lewat refresh manual
		}
	})();
	return () => {
		berhenti = true;
		try {
			channel?.unsubscribe();
		} catch {
			// abaikan
		}
	};
}

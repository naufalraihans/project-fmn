// Penjaga seluruh halaman /app. Berjalan di server sebelum halaman dirender:
// belum login -> /masuk; wajib ganti password -> /ganti-password;
// menu disusun menurut peran (superadmin/admin/user).
import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';
import { klienServer, apiBase } from '$lib/supabase-server';
import type { Peran } from '$lib/sesi';

interface Me {
	id: string;
	email: string;
	role: string;
	status: string;
	must_change_password: boolean;
}

export const load: LayoutServerLoad = async (event) => {
	const sb = klienServer(event);
	const {
		data: { session }
	} = await sb.auth.getSession();
	if (!session) throw redirect(303, '/masuk');

	// Peran & status dibaca dari backend Go (database), bukan dari klaim token,
	// sehingga penonaktifan akun langsung berlaku.
	let me: Me;
	try {
		const res = await fetch(`${apiBase()}/api/auth/me`, {
			headers: { Authorization: `Bearer ${session.access_token}` }
		});
		if (res.status === 401) throw redirect(303, '/masuk');
		if (!res.ok) throw new Error('backend');
		me = ((await res.json()) as { data: Me }).data;
	} catch (e) {
		if (e instanceof Response) throw e;
		throw redirect(303, '/masuk');
	}

	if (me.status !== 'aktif') {
		await sb.auth.signOut();
		throw redirect(303, '/masuk');
	}
	if (me.must_change_password && !event.url.pathname.startsWith('/ganti-password')) {
		throw redirect(303, '/ganti-password');
	}

	const peran = me.role as Peran;
	const menu =
		peran === 'superadmin'
			? [
					{ href: '/app/keuangan', nama: 'Keuangan' },
					{ href: '/app/invoice', nama: 'Invoice' },
					{ href: '/app/absensi', nama: 'Absensi Kru' },
					{ href: '/app/akun', nama: 'Akun' },
					{ href: '/app/katalog', nama: 'Katalog' },
					{ href: '/app/aset', nama: 'Aset' },
					{ href: '/app/konten', nama: 'Konten Compro' }
				]
			: peran === 'admin'
				? [
						{ href: '/app/absensi', nama: 'Absensi Kru' },
						{ href: '/app/akun', nama: 'Akun Kru' },
						{ href: '/app/katalog', nama: 'Katalog' },
						{ href: '/app/aset', nama: 'Aset' },
						{ href: '/app/konten', nama: 'Konten Compro' }
					]
				: [
						{ href: '/app/absen', nama: 'Absen Saya' },
						{ href: '/app/aset', nama: 'Aset Saya' }
					];

	return { peran, email: me.email, menu };
};

import { redirect } from '@sveltejs/kit';
import type { PageLoad } from './$types';

export const load: PageLoad = async ({ parent }) => {
	const { peran } = await parent();
	if (peran === 'superadmin') throw redirect(303, '/app/keuangan');
	if (peran === 'admin') throw redirect(303, '/app/absensi');
	throw redirect(303, '/app/absen');
};

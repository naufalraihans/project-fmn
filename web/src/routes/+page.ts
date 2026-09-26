import type { PageLoad } from './$types';
import { muatCompro } from '$lib/muat-compro';
import { env } from '$env/dynamic/public';

export const load: PageLoad = async () => {
	const base = env.PUBLIC_API_BASE ?? 'http://localhost:8080';
	return { compro: await muatCompro(base) };
};

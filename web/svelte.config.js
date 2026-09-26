import adapter from '@sveltejs/adapter-vercel';
import { vitePreprocess } from '@sveltejs/vite-plugin-svelte';

/** @type {import('@sveltejs/kit').Config} */
const config = {
	preprocess: vitePreprocess(),
	kit: {
		// runtime WAJIB nodejs22.x atau lebih baru. @supabase/ssr membuat
		// RealtimeClient saat klien server dibuat, dan itu membutuhkan WebSocket
		// native yang baru ada di Node 22+. Dengan nodejs20.x, SETIAP halaman
		// yang dijaga +layout.server.ts (semua /app) gagal 500 dengan pesan
		// "Node.js detected but native WebSocket not found".
		adapter: adapter({ runtime: 'nodejs22.x' }),
		alias: { $lib: './src/lib' }
	}
};

export default config;

// Struktur respons backend Go. Lihat server/internal/httpx/httpx.go:
// sukses -> { "data": T } atau { "data": T, "meta": {...} }
// gagal   -> { "error": { "code": "...", "message": "..." } }
export interface ApiErrorBody {
	code: string;
	message: string;
}

export interface PageMeta {
	page: number;
	per_page: number;
	total: number;
}

export class ApiError extends Error {
	readonly code: string;
	readonly status: number;
	constructor(code: string, message: string, status: number) {
		super(message);
		this.code = code;
		this.status = status;
	}
}

/**
 * Panggil backend Go. Path SELALU diawali /api.
 * Base URL dibaca dari PUBLIC_API_BASE (env publik).
 * Token Supabase diteruskan sebagai Bearer bila tersedia.
 */
export async function api<T>(
	path: string,
	opts: {
		method?: string;
		body?: unknown;
		token?: string | null;
		base?: string;
	} = {}
): Promise<{ data: T; meta?: PageMeta }> {
	const base = (opts.base ?? import.meta.env.PUBLIC_API_BASE ?? 'http://localhost:8080').replace(
		/\/$/,
		''
	);
	const headers: Record<string, string> = { 'Content-Type': 'application/json' };
	if (opts.token) headers['Authorization'] = `Bearer ${opts.token}`;

	const res = await fetch(`${base}${path}`, {
		method: opts.method ?? (opts.body !== undefined ? 'POST' : 'GET'),
		headers,
		body: opts.body !== undefined ? JSON.stringify(opts.body) : undefined
	});

	let payload: unknown = null;
	try {
		payload = await res.json();
	} catch {
		// respons bukan JSON: anggap gangguan server
		throw new ApiError('INTERNAL', 'Terjadi gangguan di server. Coba beberapa saat lagi.', res.status);
	}

	if (!res.ok) {
		const err = (payload as { error?: ApiErrorBody })?.error;
		throw new ApiError(
			err?.code ?? 'INTERNAL',
			err?.message ?? 'Terjadi gangguan di server. Coba beberapa saat lagi.',
			res.status
		);
	}
	return payload as { data: T; meta?: PageMeta };
}

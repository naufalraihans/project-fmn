<script lang="ts">
	import { goto } from '$app/navigation';
	import { supabase } from '$lib/supabase';
	import { api, ApiError } from '$lib/api';
	import { simpanSesi, type Peran } from '$lib/sesi';

	let email = '';
	let password = '';
	let sibuk = false;
	let galat = '';

	async function masuk() {
		sibuk = true;
		galat = '';
		try {
			const sb = supabase();
			const { data, error } = await sb.auth.signInWithPassword({ email, password });
			if (error) throw new Error(terjemah(error.message));
			const token = data.session?.access_token;
			if (!token) throw new Error('Sesi tidak terbentuk. Coba lagi.');
			const me = await api<{
				id: string;
				email: string;
				role: string;
				must_change_password: boolean;
				status: string;
			}>('/api/auth/me', { token });
			if (me.data.status !== 'aktif') {
				await sb.auth.signOut();
				throw new Error('Akun Anda dinonaktifkan. Hubungi administrator.');
			}
			if (me.data.must_change_password) {
				// Arahkan ke halaman ganti password; sesi Supabase tetap dipakai.
				simpanSesi({
					userId: me.data.id,
					email: me.data.email,
					peran: me.data.role as Peran,
					wajibGantiPassword: true
				});
				await goto('/ganti-password');
				return;
			}
			simpanSesi({
				userId: me.data.id,
				email: me.data.email,
				peran: me.data.role as Peran,
				wajibGantiPassword: false
			});
			const params = new URLSearchParams(window.location.search);
			const lanjut = params.get('lanjut');
			if (lanjut && lanjut.startsWith('/app/')) {
				await goto(lanjut);
				return;
			}
			await goto(rumah(me.data.role as Peran));
		} catch (e) {
			galat = e instanceof ApiError ? e.message : (e as Error).message;
		} finally {
			sibuk = false;
		}
	}

	function rumah(peran: Peran): string {
		if (peran === 'superadmin') return '/app/keuangan';
		if (peran === 'admin') return '/app/absensi';
		return '/app/absen';
	}

	function terjemah(pesan: string): string {
		if (/invalid login|invalid.*credentials/i.test(pesan)) return 'Email atau password salah.';
		if (/email not confirmed/i.test(pesan)) return 'Email belum dikonfirmasi. Hubungi administrator.';
		if (/too many|rate/i.test(pesan)) return 'Terlalu banyak percobaan. Tunggu sebentar lalu coba lagi.';
		return 'Gagal masuk. Periksa koneksi lalu coba lagi.';
	}
</script>

<svelte:head><title>Masuk - FMN</title></svelte:head>

<div class="wrap tengah">
	<div class="box">
		<a class="logo" href="/"><b>Focus</b><span>Management Nusantara</span></a>
		<h1>Masuk Aplikasi</h1>
		<p class="sub">Untuk kru dan staf FMN. Akun dibuat oleh administrator - tidak ada pendaftaran mandiri.</p>
		<form on:submit|preventDefault={masuk}>
			<div class="field"><label for="email">Email</label><input id="email" type="email" bind:value={email} required autocomplete="username" /></div>
			<div class="field"><label for="sandi">Password</label><input id="sandi" type="password" bind:value={password} required autocomplete="current-password" /></div>
			{#if galat}<div class="alert error">{galat}</div>{/if}
			<button class="btn" type="submit" disabled={sibuk}>{sibuk ? 'Memeriksa...' : 'Masuk'}<span class="chip" aria-hidden="true">&rarr;</span></button>
		</form>
		<p class="kembali"><a href="/">&larr; Kembali ke beranda</a></p>
	</div>
</div>

<style>
	.tengah {
		min-height: 100vh;
		display: grid;
		place-items: center;
		background: var(--ink-900);
		padding: 24px var(--gutter);
	}
	.box {
		background: #fff;
		border-radius: var(--r-lg);
		padding: 34px 30px;
		max-width: 440px;
		width: 100%;
	}
	.logo {
		display: flex;
		flex-direction: column;
		line-height: 1.1;
		margin-bottom: 18px;
	}
	.logo b {
		font: 800 20px/1 var(--font-display);
	}
	.logo span {
		font-size: 11px;
		color: var(--text-400);
		letter-spacing: 0.12em;
		text-transform: uppercase;
	}
	h1 {
		font: 800 26px/1.2 var(--font-display);
		margin-bottom: 6px;
	}
	.sub {
		font-size: 13.5px;
		color: var(--text-500);
		margin-bottom: 20px;
	}
	.btn {
		width: 100%;
		justify-content: space-between;
	}
	.kembali {
		margin-top: 18px;
		font-size: 13.5px;
		color: var(--text-500);
	}
</style>

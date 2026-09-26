<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { supabase } from '$lib/supabase';
	import { api, ApiError } from '$lib/api';
	import { sesi, simpanSesi, hapusSesi } from '$lib/sesi';

	let sandiBaru = '';
	let sandiUlang = '';
	let sibuk = false;
	let galat = '';

	onMount(async () => {
		// Halaman ini hanya relevan bila flag wajib-ganti masih menyala.
		const { data } = await supabase().auth.getSession();
		if (!data.session) {
			await goto('/masuk');
			return;
		}
		try {
			const me = await api<{ must_change_password: boolean; role: string; id: string; email: string }>(
				'/api/auth/me',
				{ token: data.session.access_token }
			);
			if (!me.data.must_change_password) {
				await goto('/app');
				return;
			}
			simpanSesi({
				userId: me.data.id,
				email: me.data.email,
				peran: me.data.role as 'superadmin' | 'admin' | 'user',
				wajibGantiPassword: true
			});
		} catch {
			await goto('/masuk');
		}
	});

	async function ganti() {
		galat = '';
		if (sandiBaru.length < 8) {
			galat = 'Password minimal 8 karakter.';
			return;
		}
		if (sandiBaru !== sandiUlang) {
			galat = 'Konfirmasi password tidak sama.';
			return;
		}
		sibuk = true;
		try {
			const sb = supabase();
			const { error } = await sb.auth.updateUser({ password: sandiBaru });
			if (error) throw new Error('Gagal mengganti password. Coba lagi.');
			const { data } = await sb.auth.getSession();
			const token = data.session?.access_token;
			if (!token) throw new Error('Sesi hilang. Masuk kembali.');
			// Beritahu backend agar flag dibuka; tanpa ini pengguna terjebak.
			await api('/api/auth/password-changed', { method: 'POST', token, body: {} });
			const s = $sesi;
			if (s) simpanSesi({ ...s, wajibGantiPassword: false });
			await goto('/app');
		} catch (e) {
			galat = e instanceof ApiError ? e.message : (e as Error).message;
		} finally {
			sibuk = false;
		}
	}

	async function keluar() {
		await supabase().auth.signOut();
		hapusSesi();
		await goto('/masuk');
	}
</script>

<svelte:head><title>Ganti Password - FMN</title></svelte:head>

<div class="wrap tengah">
	<div class="box">
		<h1>Ganti Password</h1>
		<p class="sub">Akun Anda memakai password awal. Buat password baru sebelum memakai aplikasi.</p>
		<form on:submit|preventDefault={ganti}>
			<div class="field"><label for="baru">Password baru (min. 8 karakter)</label><input id="baru" type="password" bind:value={sandiBaru} required minlength="8" autocomplete="new-password" /></div>
			<div class="field"><label for="ulang">Ulangi password baru</label><input id="ulang" type="password" bind:value={sandiUlang} required autocomplete="new-password" /></div>
			{#if galat}<div class="alert error">{galat}</div>{/if}
			<button class="btn" type="submit" disabled={sibuk}>{sibuk ? 'Menyimpan...' : 'Simpan Password Baru'}<span class="chip" aria-hidden="true">&rarr;</span></button>
		</form>
		<p class="kembali"><button on:click={keluar}>Keluar</button></p>
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
		margin-top: 14px;
		font-size: 13.5px;
		color: var(--text-500);
	}
	.kembali button {
		color: var(--blue-600);
		text-decoration: underline;
	}
</style>

<script lang="ts">
	import { onMount } from 'svelte';
	import { api, ApiError } from '$lib/api';
	import { ambilToken } from '$lib/sesi';
	import { jam, label, rupiah, tanggalPanjang } from '$lib/format';

	interface Akun {
		id: string;
		nama: string;
		email: string;
		telepon: string;
		role: string;
		status: string;
	}

	let daftar: Akun[] = [];
	let galat = '';
	let nama = '';
	let email = '';
	let buatHasil = '';
	let sibuk = false;

	async function muat() {
		galat = '';
		try {
			const token = await ambilToken();
			const r = await api<Akun[]>('/api/accounts?per_page=50', { token });
			daftar = r.data ?? [];
		} catch (e) {
			galat = e instanceof ApiError ? e.message : 'Gagal memuat daftar akun.';
		}
	}

	async function buat() {
		sibuk = true;
		buatHasil = '';
		galat = '';
		try {
			const token = await ambilToken();
			const r = await api<{ akun: Akun; password_awal: string; catatan: string }>('/api/accounts', {
				method: 'POST',
				token,
				body: { nama, email, role: 'user' }
			});
			buatHasil = `Akun ${r.data.akun.email} dibuat. Password awal (tampil sekali): ${r.data.password_awal}`;
			nama = email = '';
			await muat();
		} catch (e) {
			galat = e instanceof ApiError ? e.message : 'Pembuatan akun gagal.';
		} finally {
			sibuk = false;
		}
	}

	async function setStatus(a: Akun, status: 'aktif' | 'nonaktif') {
		try {
			const token = await ambilToken();
			await api(`/api/accounts/${a.id}/${status}`, { method: 'POST', token, body: {} });
			await muat();
		} catch (e) {
			galat = e instanceof ApiError ? e.message : 'Perubahan status gagal.';
		}
	}

	async function reset(a: Akun) {
		buatHasil = '';
		try {
			const token = await ambilToken();
			const r = await api<{ password_baru: string }>(`/api/accounts/${a.id}/reset-password`, {
				method: 'POST',
				token,
				body: {}
			});
			buatHasil = `Password baru ${a.email} (tampil sekali): ${r.data.password_baru}`;
		} catch (e) {
			galat = e instanceof ApiError ? e.message : 'Reset password gagal.';
		}
	}

	onMount(muat);
</script>

<div class="page-head"><div><h1>Akun Kru</h1><p>Buat akun, nonaktifkan yang keluar, dan reset password yang lupa.</p></div></div>

<div class="card">
	<h3>Buat Akun Kru Baru</h3>
	<form on:submit|preventDefault={buat}>
		<div class="field"><label for="nama">Nama lengkap</label><input id="nama" bind:value={nama} required minlength="2" maxlength="120" /></div>
		<div class="field"><label for="email">Email</label><input id="email" type="email" bind:value={email} required /></div>
		<button class="btn plain" type="submit" disabled={sibuk}>{sibuk ? 'Membuat...' : 'Buat Akun Kru'}</button>
	</form>
	{#if buatHasil}<div class="alert ok" style="margin-top:12px">{buatHasil}</div>{/if}
</div>

{#if galat}<div class="alert error" style="margin-top:14px">{galat}</div>{/if}

<div class="table-wrap" style="margin-top:16px">
	<table class="grid">
		<thead><tr><th>Nama</th><th>Email</th><th>Status</th><th>Aksi</th></tr></thead>
		<tbody>
			{#each daftar as a}
				<tr>
					<td>{a.nama}</td>
					<td>{a.email}</td>
					<td><span class="badge {a.status === 'aktif' ? 'ok' : 'danger'}">{label(a.status)}</span></td>
					<td class="aksi">
						{#if a.status === 'aktif'}
							<button on:click={() => setStatus(a, 'nonaktif')}>Nonaktifkan Akun Ini</button>
						{:else}
							<button on:click={() => setStatus(a, 'aktif')}>Aktifkan Kembali</button>
						{/if}
						<button on:click={() => reset(a)}>Reset Password</button>
					</td>
				</tr>
			{:else}
				<tr><td colspan="4">Belum ada akun kru.</td></tr>
			{/each}
		</tbody>
	</table>
</div>

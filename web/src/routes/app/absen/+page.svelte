<script lang="ts">
	import { onMount } from 'svelte';
	import { api, ApiError } from '$lib/api';
	import { ambilToken } from '$lib/sesi';
	import { jam, tanggalPanjang, label } from '$lib/format';

	interface Absen {
		id: string;
		tanggal: string;
		hari: string;
		check_in_at: string | null;
		check_out_at: string | null;
		durasi_menit: number | null;
		status: string;
	}

	let riwayat: Absen[] = [];
	let total = 0;
	let galat = '';
	let aksi: 'idle' | 'masuk' | 'pulang' = 'idle';
	let pesan = '';

	async function muat() {
		galat = '';
		try {
			const token = await ambilToken();
			const r = await api<Absen[]>('/api/attendance/me?per_page=14', { token });
			riwayat = r.data ?? [];
			total = r.meta?.total ?? riwayat.length;
		} catch (e) {
			galat = e instanceof ApiError ? e.message : 'Gagal memuat riwayat absen.';
		}
	}

	async function absen(jenis: 'masuk' | 'pulang') {
		aksi = jenis;
		pesan = '';
		try {
			const token = await ambilToken();
			const r = await api<Absen>(`/api/attendance/check-${jenis === 'masuk' ? 'in' : 'out'}`, {
				method: 'POST',
				token,
				body: {}
			});
			pesan = jenis === 'masuk' ? `Absen masuk tercatat ${jam(r.data.check_in_at)}.` : `Absen pulang tercatat ${jam(r.data.check_out_at)}.`;
			await muat();
		} catch (e) {
			pesan = e instanceof ApiError ? e.message : 'Absen gagal. Coba lagi.';
		} finally {
			aksi = 'idle';
		}
	}

	onMount(muat);
</script>

<div class="page-head"><div><h1>Absen Saya</h1><p>Waktu diambil dari server. Cukup satu kali masuk dan satu kali pulang per hari.</p></div></div>

<div class="aksi">
	<button class="btn" on:click={() => absen('masuk')} disabled={aksi !== 'idle'}>
		{aksi === 'masuk' ? 'Mencatat...' : 'Absen Masuk'}<span class="chip" aria-hidden="true">&rarr;</span>
	</button>
	<button class="btn secondary" on:click={() => absen('pulang')} disabled={aksi !== 'idle'}>
		{aksi === 'pulang' ? 'Mencatat...' : 'Absen Pulang'}<span class="chip" aria-hidden="true">&rarr;</span>
	</button>
</div>
{#if pesan}<div class="alert {pesan.includes('tercatat') ? 'ok' : 'error'}">{pesan}</div>{/if}
{#if galat}<div class="alert error">{galat}</div>{/if}

<div class="table-wrap">
	<table class="grid">
		<thead><tr><th>Tanggal</th><th>Masuk</th><th>Pulang</th><th>Durasi</th><th>Status</th></tr></thead>
		<tbody>
			{#each riwayat as a}
				<tr>
					<td>{tanggalPanjang(a.tanggal)}<br /><small>{a.hari}</small></td>
					<td>{jam(a.check_in_at)}</td>
					<td>{jam(a.check_out_at)}</td>
					<td>{a.durasi_menit !== null ? `${a.durasi_menit} mnt` : '-'}</td>
					<td><span class="badge {a.status === 'hadir' ? 'ok' : a.status === 'alpha' ? 'danger' : 'warn'}">{label(a.status)}</span></td>
				</tr>
			{:else}
				<tr><td colspan="5">Belum ada riwayat absen.</td></tr>
			{/each}
		</tbody>
	</table>
</div>
{#if total > riwayat.length}<p class="more">Menampilkan {riwayat.length} dari {total} catatan.</p>{/if}

<style>
	.aksi {
		display: flex;
		gap: 12px;
		margin-bottom: 16px;
		flex-wrap: wrap;
	}
	.more {
		margin-top: 10px;
		font-size: 13px;
		color: var(--text-500);
	}
	small {
		color: var(--text-400);
	}
</style>

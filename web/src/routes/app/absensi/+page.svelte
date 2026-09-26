<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { api, ApiError } from '$lib/api';
	import { ambilToken } from '$lib/sesi';
	import { jam, label, tanggalPanjang } from '$lib/format';
	import { langganan } from '$lib/realtime';

	interface Baris {
		id: string;
		user_id: string;
		user_nama: string;
		tanggal: string;
		hari: string;
		check_in_at: string | null;
		check_out_at: string | null;
		durasi_menit: number | null;
		status: string;
		dikoreksi: boolean;
	}

	let dari = new Date().toISOString().slice(0, 10);
	let daftar: Baris[] = [];
	let total = 0;
	let galat = '';
	let segar = '';

	function hariIni(): string {
		return new Date().toISOString().slice(0, 10);
	}
	if (!dari) dari = hariIni();

	async function muat() {
		galat = '';
		try {
			const token = await ambilToken();
			const r = await api<Baris[]>(`/api/attendance?from=${dari}&to=${dari}&per_page=100`, { token });
			daftar = r.data ?? [];
			total = r.meta?.total ?? daftar.length;
		} catch (e) {
			galat = e instanceof ApiError ? e.message : 'Gagal memuat rekap absensi.';
		}
	}

	let henti: (() => void) | null = null;
	onMount(() => {
		muat();
		henti = langganan('fmn:ops', () => {
			segar = 'Ada absen baru. Memuat ulang...';
			muat().finally(() => (segar = ''));
		});
	});
	onDestroy(() => henti?.());

	async function unduhCsv() {
		try {
			const token = await ambilToken();
			const base = (await import('$env/dynamic/public')).env.PUBLIC_API_BASE ?? 'http://localhost:8080';
			const res = await fetch(`${base}/api/attendance/export?from=${dari}&to=${dari}`, {
				headers: token ? { Authorization: `Bearer ${token}` } : {}
			});
			if (!res.ok) throw new Error('Unduhan gagal.');
			const blob = await res.blob();
			const a = document.createElement('a');
			a.href = URL.createObjectURL(blob);
			a.download = `rekap-absensi-${dari}.csv`;
			a.click();
			URL.revokeObjectURL(a.href);
		} catch {
			galat = 'Unduhan CSV gagal. Coba lagi.';
		}
	}
</script>

<div class="page-head">
	<div><h1>Absensi Kru</h1><p>Rekap harian seluruh kru. Memperbarui sendiri saat ada yang absen.</p></div>
	<button class="btn plain" on:click={unduhCsv}>Unduh CSV Tanggal Ini</button>
</div>

<div class="toolbar">
	<label>Tanggal <input type="date" bind:value={dari} on:change={muat} /></label>
	<button class="btn plain" on:click={muat}>Tampilkan Rekap Tanggal Ini</button>
	{#if segar}<span class="badge info">{segar}</span>{/if}
</div>
{#if galat}<div class="alert error">{galat}</div>{/if}

<div class="table-wrap">
	<table class="grid">
		<thead><tr><th>Nama Kru</th><th>Masuk</th><th>Pulang</th><th>Durasi</th><th>Status</th></tr></thead>
		<tbody>
			{#each daftar as b}
				<tr>
					<td>{b.user_nama}</td>
					<td>{jam(b.check_in_at)}</td>
					<td>{jam(b.check_out_at)}</td>
					<td>{b.durasi_menit !== null ? `${b.durasi_menit} mnt` : '-'}</td>
					<td>
						<span class="badge {b.status === 'hadir' ? 'ok' : b.status === 'alpha' ? 'danger' : 'warn'}">{label(b.status)}</span>
						{#if b.dikoreksi}<span class="badge info">koreksi</span>{/if}
					</td>
				</tr>
			{:else}
				<tr><td colspan="5">Tidak ada data pada {tanggalPanjang(dari)}.</td></tr>
			{/each}
		</tbody>
	</table>
</div>
<p class="more">{total} baris.</p>

<style>
	.more {
		margin-top: 10px;
		font-size: 13px;
		color: var(--text-500);
	}
</style>

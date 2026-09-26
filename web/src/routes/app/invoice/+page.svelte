<script lang="ts">
	import { onMount } from 'svelte';
	import { api, ApiError } from '$lib/api';
	import { ambilToken } from '$lib/sesi';
	import { label, rupiah } from '$lib/format';

	interface Invoice {
		id: string;
		nomor: string;
		klien_nama: string;
		tanggal_terbit: string;
		total: number;
		status: string;
	}

	let daftar: Invoice[] = [];
	let galat = '';
	let pesan = '';
	let klien = '';
	let tanggal = new Date().toISOString().slice(0, 10);
	let deskripsi = '';
	let qty = 1;
	let satuan = 'unit';
	let harga = 0;
	let sibuk = false;

	async function muat() {
		try {
			const token = await ambilToken();
			const r = await api<Invoice[]>('/api/invoices?per_page=30', { token });
			daftar = r.data ?? [];
		} catch (e) {
			galat = e instanceof ApiError ? e.message : 'Gagal memuat invoice.';
		}
	}

	async function buat() {
		sibuk = true;
		pesan = '';
		try {
			const token = await ambilToken();
			await api('/api/invoices', {
				method: 'POST',
				token,
				body: {
					klien_nama: klien,
					tanggal_terbit: tanggal,
					baris: [{ deskripsi, qty, satuan, harga_satuan: harga }]
				}
			});
			pesan = 'Invoice draf dibuat.';
			klien = deskripsi = '';
			qty = 1;
			harga = 0;
			await muat();
		} catch (e) {
			pesan = e instanceof ApiError ? e.message : 'Pembuatan invoice gagal.';
		} finally {
			sibuk = false;
		}
	}

	async function aksi(id: string, jenis: 'issue' | 'pay' | 'cancel') {
		pesan = '';
		try {
			const token = await ambilToken();
			const body = jenis === 'pay' ? { metode: 'transfer' } : jenis === 'cancel' ? { alasan: 'Dibatalkan lewat aplikasi' } : {};
			await api(`/api/invoices/${id}/${jenis}`, { method: 'POST', token, body });
			pesan = jenis === 'issue' ? 'Invoice diterbitkan.' : jenis === 'pay' ? 'Invoice ditandai lunas.' : 'Invoice dibatalkan.';
			await muat();
		} catch (e) {
			pesan = e instanceof ApiError ? e.message : 'Aksi gagal.';
		}
	}

	onMount(muat);
</script>

<div class="page-head"><div><h1>Invoice</h1><p>Buat draf, terbitkan, tandai lunas, atau batalkan. Angka dihitung server.</p></div></div>

<div class="card">
	<h3>Buat Invoice Draf</h3>
	<form on:submit|preventDefault={buat}>
		<div class="field"><label for="klien">Nama klien</label><input id="klien" bind:value={klien} required minlength="2" /></div>
		<div class="field"><label for="tgl">Tanggal terbit</label><input id="tgl" type="date" bind:value={tanggal} required /></div>
		<div class="field"><label for="desk">Deskripsi pekerjaan</label><input id="desk" bind:value={deskripsi} required /></div>
		<div class="baris3">
			<div class="field"><label for="qty">Jumlah</label><input id="qty" type="number" bind:value={qty} min="1" required /></div>
			<div class="field"><label for="sat">Satuan</label><input id="sat" bind:value={satuan} required /></div>
			<div class="field"><label for="hrg">Harga satuan (Rp)</label><input id="hrg" type="number" bind:value={harga} min="0" required /></div>
		</div>
		<button class="btn plain" type="submit" disabled={sibuk}>{sibuk ? 'Menyimpan...' : 'Buat Draf Invoice'}</button>
	</form>
	{#if pesan}<div class="alert {pesan.includes('gagal') || pesan.includes('Gagal') ? 'error' : 'ok'}" style="margin-top:12px">{pesan}</div>{/if}
</div>

{#if galat}<div class="alert error" style="margin-top:14px">{galat}</div>{/if}

<div class="table-wrap" style="margin-top:16px">
	<table class="grid">
		<thead><tr><th>Nomor</th><th>Klien</th><th>Total</th><th>Status</th><th>Aksi</th></tr></thead>
		<tbody>
			{#each daftar as v}
				<tr>
					<td>{v.nomor}</td>
					<td>{v.klien_nama}</td>
					<td>{rupiah(v.total)}</td>
					<td><span class="badge {v.status === 'dibayar' ? 'ok' : v.status === 'batal' ? 'danger' : 'warn'}">{label(v.status)}</span></td>
					<td class="aksi">
						{#if v.status === 'draft'}
							<button on:click={() => aksi(v.id, 'issue')}>Terbitkan Invoice Ini</button>
							<button on:click={() => aksi(v.id, 'cancel')}>Batalkan Invoice Ini</button>
						{:else if v.status === 'terkirim'}
							<button on:click={() => aksi(v.id, 'pay')}>Tandai Lunas</button>
							<button on:click={() => aksi(v.id, 'cancel')}>Batalkan Invoice Ini</button>
						{/if}
					</td>
				</tr>
			{:else}
				<tr><td colspan="5">Belum ada invoice.</td></tr>
			{/each}
		</tbody>
	</table>
</div>

<style>
	.baris3 {
		display: grid;
		grid-template-columns: 1fr 1fr 1.4fr;
		gap: 12px;
	}
	.aksi button {
		display: block;
		font-size: 13px;
		color: var(--blue-600);
		text-decoration: underline;
		text-align: left;
		padding: 2px 0;
	}
</style>

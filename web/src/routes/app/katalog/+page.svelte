<script lang="ts">
	import { onMount } from 'svelte';
	import { api, ApiError } from '$lib/api';
	import { ambilToken } from '$lib/sesi';
	import { label, rupiah } from '$lib/format';

	interface Item {
		id: string;
		kode: string;
		nama: string;
		kategori: string;
		satuan: string;
		harga_satuan: number | null;
		aktif: boolean;
	}

	let daftar: Item[] = [];
	let galat = '';
	let nama = '';
	let kategori = 'sound';
	let satuan = 'unit';
	let harga: number | null = null;
	let sibuk = false;

	async function muat() {
		try {
			const token = await ambilToken();
			const r = await api<Item[]>('/api/catalog/items?per_page=50', { token });
			daftar = r.data ?? [];
		} catch (e) {
			galat = e instanceof ApiError ? e.message : 'Gagal memuat katalog.';
		}
	}

	async function buat() {
		sibuk = true;
		try {
			const token = await ambilToken();
			await api('/api/catalog/items', {
				method: 'POST',
				token,
				body: { nama, kategori, satuan, harga_satuan: harga, aktif: true, tampil_publik: false }
			});
			nama = satuan = 'unit';
			harga = null;
			await muat();
		} catch (e) {
			galat = e instanceof ApiError ? e.message : 'Penambahan item gagal.';
		} finally {
			sibuk = false;
		}
	}

	async function toggle(i: Item) {
		try {
			const token = await ambilToken();
			await api(`/api/catalog/items/${i.id}/${i.aktif ? 'nonaktif' : 'aktif'}`, {
				method: 'POST',
				token,
				body: {}
			});
			await muat();
		} catch (e) {
			galat = e instanceof ApiError ? e.message : 'Perubahan status gagal.';
		}
	}

	onMount(muat);
</script>

<div class="page-head"><div><h1>Katalog</h1><p>Daftar harga dan jasa untuk bahan invoice.</p></div></div>

<div class="card">
	<h3>Tambah Item Katalog</h3>
	<form on:submit|preventDefault={buat}>
		<div class="field"><label for="nama">Nama item</label><input id="nama" bind:value={nama} required minlength="2" /></div>
		<div class="baris3">
			<div class="field"><label for="kat">Kategori</label>
				<select id="kat" bind:value={kategori}>
					<option value="rigging_stage">Rigging &amp; Stage</option>
					<option value="sound">Sound System</option>
					<option value="lighting">Lighting</option>
					<option value="led_screen">LED / Videotron</option>
					<option value="genset">Genset</option>
					<option value="lain_lain">Lainnya</option>
				</select>
			</div>
			<div class="field"><label for="sat">Satuan</label><input id="sat" bind:value={satuan} required /></div>
			<div class="field"><label for="hrg">Harga satuan (Rp)</label><input id="hrg" type="number" bind:value={harga} min="0" /></div>
		</div>
		<button class="btn plain" type="submit" disabled={sibuk}>{sibuk ? 'Menyimpan...' : 'Tambah Item Ini'}</button>
	</form>
</div>

{#if galat}<div class="alert error" style="margin-top:14px">{galat}</div>{/if}

<div class="table-wrap" style="margin-top:16px">
	<table class="grid">
		<thead><tr><th>Nama</th><th>Kategori</th><th>Harga</th><th>Status</th><th>Aksi</th></tr></thead>
		<tbody>
			{#each daftar as i}
				<tr>
					<td>{i.nama}</td>
					<td>{label(i.kategori)}</td>
					<td>{rupiah(i.harga_satuan)}</td>
					<td><span class="badge {i.aktif ? 'ok' : 'danger'}">{label(i.aktif ? 'aktif' : 'nonaktif')}</span></td>
					<td><button class="link" on:click={() => toggle(i)}>{i.aktif ? 'Nonaktifkan Item Ini' : 'Aktifkan Item Ini'}</button></td>
				</tr>
			{:else}
				<tr><td colspan="5">Katalog masih kosong.</td></tr>
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
	.link {
		font-size: 13px;
		color: var(--blue-600);
		text-decoration: underline;
	}
</style>

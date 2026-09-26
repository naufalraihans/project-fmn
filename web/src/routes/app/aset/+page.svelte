<script lang="ts">
	import { onMount } from 'svelte';
	import { api, ApiError } from '$lib/api';
	import { ambilToken, sesi } from '$lib/sesi';
	import { label, rupiah } from '$lib/format';

	interface Aset {
		id: string;
		kode_aset: string;
		nama: string;
		kategori: string;
		jumlah_total: number;
		jumlah_tersedia: number;
		lokasi: string;
		kondisi: string;
		status: string;
	}

	interface Event {
		id: string;
		nama_event: string;
	}

	let daftar: Aset[] = [];
	let galat = '';
	let pesan = '';
	let nama = '';
	let kategori = 'sound';
	let jumlah = 1;
	let lokasi = 'Gudang A';
	let sibuk = false;

	// Form pemakaian
	let pakaiId: string | null = null;
	let eventId = '';
	let qty = 1;
	let pj = '';
	let daftarEvent: Event[] = [];

	async function muat() {
		try {
			const token = await ambilToken();
			const r = await api<Aset[]>('/api/assets?per_page=50', { token });
			daftar = r.data ?? [];
		} catch (e) {
			galat = e instanceof ApiError ? e.message : 'Gagal memuat aset.';
		}
	}

	async function muatEvent() {
		try {
			const token = await ambilToken();
			const r = await api<Event[]>('/api/events', { token });
			daftarEvent = r.data ?? [];
		} catch {
			// kru tidak boleh melihat event; form pemakaian disembunyikan
		}
	}

	async function buat() {
		sibuk = true;
		try {
			const token = await ambilToken();
			await api('/api/assets', {
				method: 'POST',
				token,
				body: { nama, kategori, jumlah_total: jumlah, lokasi, kondisi: 'baik' }
			});
			nama = '';
			jumlah = 1;
			await muat();
		} catch (e) {
			galat = e instanceof ApiError ? e.message : 'Penambahan aset gagal.';
		} finally {
			sibuk = false;
		}
	}

	async function pakai() {
		if (!pakaiId || !eventId) return;
		pesan = '';
		try {
			const token = await ambilToken();
			await api(`/api/assets/${pakaiId}/usages`, {
				method: 'POST',
				token,
				body: { event_id: eventId, qty, penanggung_jawab: pj }
			});
			pesan = 'Pemakaian aset tercatat, stok berkurang.';
			pakaiId = null;
			await muat();
		} catch (e) {
			pesan = e instanceof ApiError ? e.message : 'Pencatatan pemakaian gagal.';
		}
	}

	onMount(() => {
		muat();
		muatEvent();
	});
</script>

<div class="page-head"><div><h1>Aset</h1><p>Stok peralatan, pemakaian per event, dan pengembalian.</p></div></div>

{#if $sesi && $sesi.peran !== 'user'}
	<div class="card">
		<h3>Tambah Aset Baru</h3>
		<form on:submit|preventDefault={buat}>
			<div class="field"><label for="nama">Nama aset</label><input id="nama" bind:value={nama} required minlength="2" /></div>
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
				<div class="field"><label for="jml">Jumlah</label><input id="jml" type="number" bind:value={jumlah} min="0" /></div>
				<div class="field"><label for="lok">Lokasi</label><input id="lok" bind:value={lokasi} /></div>
			</div>
			<button class="btn plain" type="submit" disabled={sibuk}>{sibuk ? 'Menyimpan...' : 'Tambah Aset Ini'}</button>
		</form>
	</div>
{/if}

{#if galat}<div class="alert error" style="margin-top:14px">{galat}</div>{/if}
{#if pesan}<div class="alert ok" style="margin-top:14px">{pesan}</div>{/if}

<div class="table-wrap" style="margin-top:16px">
	<table class="grid">
		<thead><tr><th>Nama</th><th>Kategori</th><th>Tersedia</th><th>Kondisi</th><th>Status</th><th>Aksi</th></tr></thead>
		<tbody>
			{#each daftar as a}
				<tr>
					<td>{a.nama}<br /><small>{a.lokasi}</small></td>
					<td>{label(a.kategori)}</td>
					<td>{a.jumlah_tersedia} dari {a.jumlah_total}</td>
					<td>{label(a.kondisi)}</td>
					<td><span class="badge {a.status === 'tersedia' ? 'ok' : a.status === 'perawatan' ? 'danger' : 'warn'}">{label(a.status)}</span></td>
					<td>
						{#if $sesi && $sesi.peran !== 'user' && daftarEvent.length}
							<button class="link" on:click={() => (pakaiId = a.id)}>Catat Pemakaian</button>
						{/if}
					</td>
				</tr>
			{:else}
				<tr><td colspan="6">Belum ada aset.</td></tr>
			{/each}
		</tbody>
	</table>
</div>

{#if pakaiId}
	<div class="card" style="margin-top:16px">
		<h3>Catat Pemakaian Aset</h3>
		<form on:submit|preventDefault={pakai}>
			<div class="field"><label for="ev">Event</label>
				<select id="ev" bind:value={eventId} required>
					<option value="">Pilih event</option>
					{#each daftarEvent as e}<option value={e.id}>{e.nama_event}</option>{/each}
				</select>
			</div>
			<div class="baris3">
				<div class="field"><label for="qty">Jumlah keluar</label><input id="qty" type="number" bind:value={qty} min="1" required /></div>
				<div class="field"><label for="pj">Penanggung jawab</label><input id="pj" bind:value={pj} required /></div>
			</div>
			<button class="btn plain" type="submit">Simpan Pemakaian Ini</button>
			<button type="button" class="link" on:click={() => (pakaiId = null)}>Batal</button>
		</form>
	</div>
{/if}

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
		margin-left: 8px;
	}
	small {
		color: var(--text-400);
	}
</style>

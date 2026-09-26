<script lang="ts">
	import { onMount } from 'svelte';
	import { api, ApiError } from '$lib/api';
	import { ambilToken } from '$lib/sesi';

	interface Inquiry {
		id: string;
		nama: string;
		kontak: string;
		jenis_kebutuhan: string;
		pesan: string;
		handled: boolean;
		received_at: string;
	}

	interface Blok {
		key: string;
		isi: Record<string, unknown>;
		published: boolean;
	}

	let tab: 'masuk' | 'konten' = 'masuk';
	let daftar: Inquiry[] = [];
	let blok: Blok[] = [];
	let galat = '';
	let kunciUbah: Record<string, string> = {};

	async function muat() {
		try {
			const token = await ambilToken();
			const [inq, ctn] = await Promise.all([
				api<Inquiry[]>('/api/inquiries?per_page=30', { token }),
				api<Blok[]>('/api/content', { token }).catch(() => ({ data: [] }))
			]);
			daftar = (inq.data ?? []).filter((x) => !x.handled);
			blok = ctn.data ?? [];
		} catch (e) {
			galat = e instanceof ApiError ? e.message : 'Gagal memuat konten.';
		}
	}

	async function tandai(i: Inquiry) {
		try {
			const token = await ambilToken();
			await api(`/api/inquiries/${i.id}/handled`, { method: 'POST', token, body: {} });
			daftar = daftar.filter((x) => x.id !== i.id);
		} catch (e) {
			galat = e instanceof ApiError ? e.message : 'Gagal menandai pesan.';
		}
	}

	async function simpan(b: Blok) {
		try {
			const token = await ambilToken();
			const isi = JSON.parse(kunciUbah[b.key] ?? JSON.stringify(b.isi, null, 2));
			await api(`/api/content/${b.key}`, { method: 'PUT', token, body: { isi } });
			b.isi = isi;
			delete kunciUbah[b.key];
		} catch {
			galat = `Penyimpanan blok ${b.key} gagal. Pastikan isinya JSON yang valid.`;
		}
	}

	onMount(muat);
</script>

<div class="page-head"><div><h1>Konten Compro</h1><p>Pesan masuk dari formulir dan teks halaman depan.</p></div></div>
{#if galat}<div class="alert error">{galat}</div>{/if}

<div class="toolbar">
	<button class="btn plain" on:click={() => (tab = 'masuk')}>Pesan Masuk ({daftar.length})</button>
	<button class="btn plain" on:click={() => (tab = 'konten')}>Teks Halaman Depan</button>
</div>

{#if tab === 'masuk'}
	<div class="table-wrap">
		<table class="grid">
			<thead><tr><th>Nama</th><th>Kontak</th><th>Pesan</th><th>Aksi</th></tr></thead>
			<tbody>
				{#each daftar as i}
					<tr>
						<td>{i.nama}</td>
						<td>{i.kontak}</td>
						<td>{i.pesan}</td>
						<td><button class="link" on:click={() => tandai(i)}>Tandai Sudah Ditangani</button></td>
					</tr>
				{:else}
					<tr><td colspan="4">Tidak ada pesan baru.</td></tr>
				{/each}
			</tbody>
		</table>
	</div>
{:else}
	{#each blok as b}
		<div class="card" style="margin-bottom:12px">
			<h3>Blok {b.key}</h3>
			<div class="field">
				<label for="isi-{b.key}">Isi (format JSON)</label>
				<textarea id="isi-{b.key}" rows="6" bind:value={kunciUbah[b.key]} placeholder={JSON.stringify(b.isi, null, 2)}></textarea>
			</div>
			<button class="btn plain" on:click={() => simpan(b)}>Simpan Blok {b.key}</button>
		</div>
	{:else}
		<p>Belum ada blok konten.</p>
	{/each}
{/if}

<style>
	.link {
		font-size: 13px;
		color: var(--blue-600);
		text-decoration: underline;
	}
</style>

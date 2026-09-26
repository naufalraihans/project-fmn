<script lang="ts">
	import { onMount } from 'svelte';
	import { api, ApiError } from '$lib/api';
	import { ambilToken } from '$lib/sesi';
	import { label, rupiah } from '$lib/format';

	interface Ringkas {
		periode: string;
		jumlah_invoice: number;
		jumlah_batal: number;
		terbit: number;
		dibayar: number;
		piutang: number;
		batal: number;
	}

	let per: Ringkas[] = [];
	let total: Ringkas | null = null;
	let galat = '';

	async function muat() {
		try {
			const token = await ambilToken();
			const r = await api<{ per_periode: Ringkas[]; total: Ringkas }>('/api/finance/summary', { token });
			per = r.data.per_periode ?? [];
			total = r.data.total;
		} catch (e) {
			galat = e instanceof ApiError ? e.message : 'Gagal memuat neraca.';
		}
	}

	onMount(muat);
</script>

<div class="page-head"><div><h1>Keuangan</h1><p>Neraca dihitung otomatis dari invoice. Tidak ada angka yang diisi manual.</p></div></div>
{#if galat}<div class="alert error">{galat}</div>{/if}

{#if total}
	<div class="kartu">
		<div class="k"><span>Total Terbit</span><b>{rupiah(total.terbit)}</b><small>{total.jumlah_invoice} invoice</small></div>
		<div class="k"><span>Sudah Lunas</span><b class="ok">{rupiah(total.dibayar)}</b></div>
		<div class="k"><span>Piutang</span><b class="warn">{rupiah(total.piutang)}</b></div>
		<div class="k"><span>Batal</span><b>{rupiah(total.batal)}</b><small>{total.jumlah_batal} invoice</small></div>
	</div>
{/if}

<div class="table-wrap" style="margin-top:16px">
	<table class="grid">
		<thead><tr><th>Periode</th><th>Invoice</th><th>Terbit</th><th>Lunas</th><th>Piutang</th><th>Batal</th></tr></thead>
		<tbody>
			{#each per as p}
				<tr>
					<td>{p.periode}</td>
					<td>{p.jumlah_invoice}</td>
					<td>{rupiah(p.terbit)}</td>
					<td>{rupiah(p.dibayar)}</td>
					<td>{rupiah(p.piutang)}</td>
					<td>{rupiah(p.batal)}</td>
				</tr>
			{:else}
				<tr><td colspan="6">Belum ada invoice.</td></tr>
			{/each}
		</tbody>
	</table>
</div>

<style>
	.kartu {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 12px;
	}
	.k {
		border: 1px solid var(--line-light);
		border-radius: var(--r-md);
		padding: 16px;
		display: grid;
		gap: 4px;
	}
	.k span {
		font-size: 12.5px;
		color: var(--text-500);
	}
	.k b {
		font: 800 20px/1.2 var(--font-display);
	}
	.k b.ok {
		color: var(--ok);
	}
	.k b.warn {
		color: var(--warn);
	}
	.k small {
		font-size: 12px;
		color: var(--text-400);
	}
	@media (max-width: 820px) {
		.kartu {
			grid-template-columns: 1fr 1fr;
		}
	}
</style>

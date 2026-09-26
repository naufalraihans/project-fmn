<script lang="ts">
	import type { PageData } from './$types';
	import KartuLayanan from '$lib/KartuLayanan.svelte';
	import { teks } from '$lib/compro';
	import { api, ApiError } from '$lib/api';

	let { data }: { data: PageData } = $props();
	const { compro } = data;

	const LAYANAN_BAWAAN = [
		{ nama: 'LED Videotron', deskripsi: 'Rental LED hybrid indoor/outdoor P3.9 dengan kualitas terbaik.', cakupan: ['Indoor', 'Outdoor', 'Processor'] },
		{ nama: 'Sound System', deskripsi: 'Line array, FOH, monitor, hingga kebutuhan audio skala besar.', cakupan: ['Line array', 'FOH', 'Monitor'] },
		{ nama: 'Lighting', deskripsi: 'Beam, parled, fresnel, follow spot dan lighting console profesional.', cakupan: ['Beam', 'Parled', 'Console'] },
		{ nama: 'Stage & Rigging', deskripsi: 'Panggung, rigging, barikade dan struktur event yang aman dan presisi.', cakupan: ['Panggung', 'Truss', 'Barikade'] }
	];
	const layanan = compro.layanan.length ? compro.layanan : LAYANAN_BAWAAN;

	let menuBuka = false;
	let nama = '';
	let kontak = '';
	let pesan = '';
	let kirimStatus: 'idle' | 'kirim' | 'ok' | 'gagal' = 'idle';
	let kirimPesan = '';

	async function kirimInquiry() {
		kirimStatus = 'kirim';
		kirimPesan = '';
		try {
			await api('/api/public/inquiries', { body: { nama, kontak, pesan } });
			kirimStatus = 'ok';
			kirimPesan = 'Pesan terkirim. Tim kami akan menghubungi Anda.';
			nama = kontak = pesan = '';
		} catch (e) {
			kirimStatus = 'gagal';
			kirimPesan = e instanceof ApiError ? e.message : 'Pengiriman gagal. Coba lagi.';
		}
	}
</script>

<svelte:head>
	<title>Focus Management Nusantara - Event Production Partner</title>
</svelte:head>

<header class="site">
	<div class="wrap">
		<a class="logo" href="#beranda"><b>Focus</b><span>Management Nusantara</span></a>
		<button class="menu-btn" aria-expanded={menuBuka} on:click={() => (menuBuka = !menuBuka)}>Menu</button>
		<nav class="main" class:open={menuBuka}>
			<a href="#beranda">Beranda</a>
			<a href="#tentang">Tentang Kami</a>
			<a href="#layanan">Layanan</a>
			<a href="#portofolio">Portofolio</a>
			<a href="#inventori">Inventori</a>
			<a href="#kontak">Kontak</a>
		</nav>
		<a class="btn" href="#kontak">Konsultasi Proyek<span class="chip" aria-hidden="true">&rarr;</span></a>
	</div>
</header>

<div class="hero" id="beranda">
	<div class="wrap">
		<div class="inner">
			<span class="eyebrow">Event Production Partner</span>
			<h1>{teks(compro.hero, 'judul', 'Ide Besar Layak Dapat Panggung Terbaik')}</h1>
			<p class="lead">
				{teks(
					compro.hero,
					'deskripsi',
					'Focus Management Nusantara adalah perusahaan event production yang menyediakan solusi lengkap untuk kebutuhan acara Anda, mulai dari LED videotron, sound system, lighting, stage, rigging hingga produksi konten.'
				)}
			</p>
			<div class="actions">
				<a class="btn" href="#kontak">Diskusikan Proyek Anda<span class="chip" aria-hidden="true">&rarr;</span></a>
				<a class="btn ghost" href="#portofolio">Lihat Portofolio<span class="chip" aria-hidden="true">&#9654;</span></a>
			</div>
		</div>
	</div>
</div>

<section class="dark" id="layanan">
	<div class="wrap">
		<span class="eyebrow">Layanan</span>
		<h2 class="on-dark">Satu Tim untuk Seluruh Kebutuhan Teknis Acara</h2>
		<span class="rule"></span>
		<div class="svc">
			{#each layanan as l}
				<KartuLayanan nama={l.nama} deskripsi={l.deskripsi} cakupan={l.cakupan ?? []} />
			{/each}
		</div>
	</div>
</section>

<section class="dark2" id="portofolio">
	<div class="wrap">
		<span class="eyebrow">Portofolio</span>
		<h2 class="on-dark">Berbagai Event, Satu Standar Kualitas</h2>
		{#if compro.portofolio.length}
			<div class="gal">
				{#each compro.portofolio as p}
					<div class="tile">
						<span class="cap"><em>{p.lokasi}{p.tahun ? ` - ${p.tahun}` : ''}</em><b>{p.nama_event}</b></span>
					</div>
				{/each}
			</div>
		{:else}
			<div class="gal">
				{#each ['Blitar Djadoel', 'Hakordia Kota Blitar', 'DKV ISI Yogyakarta', 'Soekarno Coffee Fest'] as t}
					<div class="tile"><span class="cap"><b>{t}</b></span></div>
				{/each}
			</div>
		{/if}
	</div>
</section>

<section class="light" id="inventori">
	<div class="wrap">
		<span class="eyebrow gray">Inventori</span>
		<h2>Peralatan yang Kami Operasikan</h2>
		<span class="rule"></span>
		{#if compro.peralatan.length}
			<div class="inv">
				{#each compro.peralatan as e}
					<div class="inv-col"><h4>{e.nama}</h4><p>{e.keterangan}</p></div>
				{/each}
			</div>
		{:else}
			<div class="inv">
				<div class="inv-col"><h4>LED Videotron</h4><ul><li>Panel indoor P3.9</li><li>Panel outdoor</li><li>Video processor</li></ul></div>
				<div class="inv-col"><h4>Sound System</h4><ul><li>Line array</li><li>Subwoofer</li><li>Mixing console</li></ul></div>
				<div class="inv-col"><h4>Lighting</h4><ul><li>Beam &amp; parled</li><li>Fresnel</li><li>Follow spot</li></ul></div>
				<div class="inv-col"><h4>Stage &amp; Rigging</h4><ul><li>Rangka panggung</li><li>Truss</li><li>Barikade</li></ul></div>
				<div class="inv-col"><h4>Kelistrikan</h4><ul><li>Genset silent</li><li>Panel distribusi</li></ul></div>
				<div class="inv-col"><h4>Produksi Konten</h4><ul><li>Multi-camera</li><li>Live switching</li></ul></div>
			</div>
		{/if}
	</div>
</section>

<section class="off" id="tentang">
	<div class="wrap">
		<span class="eyebrow gray">Kenapa Memilih</span>
		<h2>Focus Management Nusantara?</h2>
		<span class="rule"></span>
		<p class="tentang">{teks(compro.tentang, 'isi', teks(compro.tentang, 'deskripsi', 'Event production partner untuk mewujudkan ide besar Anda. Didukung tim profesional, peralatan modern, dan pengalaman di berbagai skala event di Indonesia.'))}</p>
	</div>
</section>

<footer id="kontak">
	<div class="wrap">
		<div class="fgrid">
			<div>
				<h5>Kirim Pesan</h5>
				<form on:submit|preventDefault={kirimInquiry}>
					<div class="field"><label for="nama">Nama</label><input id="nama" bind:value={nama} required minlength="2" maxlength="120" /></div>
					<div class="field"><label for="kontak">Kontak (WA / email)</label><input id="kontak" bind:value={kontak} required minlength="6" maxlength="160" /></div>
					<div class="field"><label for="pesan">Kebutuhan acara</label><textarea id="pesan" bind:value={pesan} required minlength="10" maxlength="2000" rows="4"></textarea></div>
					{#if kirimStatus === 'ok'}<div class="alert ok">{kirimPesan}</div>{/if}
					{#if kirimStatus === 'gagal'}<div class="alert error">{kirimPesan}</div>{/if}
					<button class="btn" type="submit" disabled={kirimStatus === 'kirim'}>
						{kirimStatus === 'kirim' ? 'Mengirim...' : 'Kirim Pesan'}<span class="chip" aria-hidden="true">&rarr;</span>
					</button>
				</form>
			</div>
			<div>
				<h5>Kontak Kami</h5>
				<p><b>Blitar, Jawa Timur</b><br /><span>Melayani seluruh Indonesia</span></p>
				<p><b>+62 852-3857-8771</b><br /><span>WhatsApp</span></p>
				<p><b>focusmanagementnusantara@gmail.com</b><br /><span>Email</span></p>
				<p><a href="https://www.instagram.com/fmn.id/" target="_blank" rel="noopener">Instagram @fmn.id</a></p>
			</div>
		</div>
		<div class="legal">
			<span>&copy; 2026 Focus Management Nusantara. All rights reserved.</span>
			<span>Bigger Ideas, Brighter Events.</span>
		</div>
	</div>
</footer>

<style>
	header.site {
		position: sticky;
		top: 0;
		z-index: 20;
		background: var(--ink-800);
		color: #fff;
	}
	header.site .wrap {
		height: 84px;
		display: flex;
		align-items: center;
		gap: 32px;
	}
	.logo {
		display: flex;
		flex-direction: column;
		line-height: 1.1;
	}
	.logo b {
		font: 800 19px/1 var(--font-display);
	}
	.logo span {
		font-size: 11px;
		color: var(--text-300);
		letter-spacing: 0.12em;
		text-transform: uppercase;
	}
	nav.main {
		display: flex;
		gap: 30px;
		margin-left: auto;
	}
	nav.main a {
		font-size: 14px;
		color: #dbe4ef;
	}
	.menu-btn {
		display: none;
		margin-left: auto;
		color: #fff;
		border: 1px solid var(--line-dark);
		border-radius: var(--r-sm);
		padding: 8px 14px;
	}
	.hero {
		background: var(--ink-900);
		color: #fff;
		padding: 90px 0 70px;
	}
	.hero h1 {
		font: 800 clamp(34px, 4.2vw, 58px)/1.06 var(--font-display);
		text-transform: uppercase;
		margin: 14px 0 18px;
		white-space: pre-line;
	}
	.actions {
		display: flex;
		gap: 14px;
		margin-top: 28px;
		flex-wrap: wrap;
	}
	section {
		padding: 72px 0;
	}
	section.dark {
		background: var(--ink-800);
	}
	section.dark2 {
		background: var(--ink-700);
	}
	h2 {
		font: 700 clamp(23px, 2.4vw, 30px)/1.25 var(--font-display);
		margin-top: 10px;
	}
	h2.on-dark {
		color: #fff;
	}
	.svc {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 18px;
		margin-top: 28px;
	}
	.gal {
		display: grid;
		grid-template-columns: repeat(4, 1fr);
		gap: 16px;
		margin-top: 28px;
	}
	.tile {
		min-height: 250px;
		border-radius: var(--r-lg);
		background: linear-gradient(160deg, var(--ink-600), var(--navy-500));
		display: flex;
		align-items: flex-end;
		padding: 18px;
		border: 1px solid var(--line-dark);
	}
	.tile .cap em {
		display: block;
		font: 500 12px/1.4 var(--font-body);
		color: var(--blue-300);
		font-style: normal;
	}
	.tile .cap b {
		color: #fff;
		font: 700 17px/1.3 var(--font-display);
	}
	section.light {
		background: #fff;
	}
	.inv {
		display: grid;
		grid-template-columns: repeat(3, 1fr);
		gap: 22px;
		margin-top: 28px;
	}
	.inv-col h4 {
		font: 700 16px/1.4 var(--font-display);
		margin-bottom: 8px;
	}
	.inv-col ul {
		list-style: none;
		color: var(--text-500);
		font-size: 14px;
	}
	section.off {
		background: var(--off-white);
	}
	.tentang {
		margin-top: 16px;
		max-width: 720px;
		color: var(--text-500);
	}
	footer {
		background: var(--ink-900);
		color: #dbe4ef;
		padding: 56px 0 26px;
	}
	.fgrid {
		display: grid;
		grid-template-columns: 1.4fr 1fr;
		gap: 40px;
	}
	footer h5 {
		font: 700 15px/1.4 var(--font-display);
		color: #fff;
		margin-bottom: 14px;
	}
	footer .field label {
		color: #dbe4ef;
	}
	.legal {
		display: flex;
		justify-content: space-between;
		margin-top: 34px;
		padding-top: 18px;
		border-top: 1px solid var(--line-dark);
		font-size: 13px;
		color: var(--text-400);
		flex-wrap: wrap;
		gap: 8px;
	}
	@media (max-width: 1024px) {
		.svc {
			grid-template-columns: 1fr 1fr;
		}
		.gal {
			grid-template-columns: 1fr 1fr;
		}
		.inv {
			grid-template-columns: 1fr 1fr;
		}
		.fgrid {
			grid-template-columns: 1fr;
		}
	}
	@media (max-width: 820px) {
		nav.main {
			display: none;
			position: absolute;
			top: 84px;
			left: 0;
			right: 0;
			background: var(--ink-800);
			flex-direction: column;
			gap: 0;
			padding: 8px var(--gutter) 20px;
		}
		nav.main.open {
			display: flex;
		}
		.menu-btn {
			display: block;
		}
		header.site .btn {
			display: none;
		}
		.svc,
		.gal,
		.inv {
			grid-template-columns: 1fr;
		}
	}
</style>

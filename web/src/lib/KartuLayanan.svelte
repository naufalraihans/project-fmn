<script lang="ts">
	let {
		nama,
		deskripsi,
		cakupan = [],
		foto = null
	}: { nama: string; deskripsi: string; cakupan?: string[]; foto?: string | null } = $props();
</script>

<article class="card">
	{#if foto}
		<div class="foto"><img src={foto} alt={nama} loading="lazy" /></div>
	{/if}
	<div class="body">
		<h3>{nama}</h3>
		<p>{deskripsi}</p>
		{#if cakupan.length}
			<ul>
				{#each cakupan as c}
					<li>{c}</li>
				{/each}
			</ul>
		{/if}
		<a class="link-arrow" href="#kontak">Selengkapnya <span class="a">&rarr;</span></a>
	</div>
</article>

<style>
	.card {
		background: var(--ink-700);
		border: 1px solid var(--line-dark);
		border-radius: var(--r-lg);
		overflow: hidden;
	}
	.card .foto img {
		width: 100%;
		height: 170px;
		object-fit: cover;
	}
	.body {
		padding: 22px;
	}
	h3 {
		font: 700 18px/1.3 var(--font-display);
		color: #fff;
		margin-bottom: 8px;
	}
	p {
		font-size: 14px;
		color: var(--text-300);
		margin-bottom: 10px;
	}
	ul {
		list-style: none;
		margin: 0 0 12px;
		padding: 0;
		display: flex;
		flex-wrap: wrap;
		gap: 6px;
	}
	li {
		font-size: 12.5px;
		color: var(--text-300);
		border: 1px solid var(--line-dark);
		border-radius: var(--r-pill);
		padding: 4px 10px;
	}
	.link-arrow {
		font: 600 12.5px/1 var(--font-body);
		color: #e6ecf3;
		display: inline-flex;
		gap: 8px;
		align-items: center;
	}
	.link-arrow:hover {
		color: #fff;
	}
</style>

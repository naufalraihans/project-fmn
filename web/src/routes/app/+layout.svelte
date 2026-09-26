<script lang="ts">
	import { goto } from '$app/navigation';
	import { onMount } from 'svelte';
	import { supabase } from '$lib/supabase';
	import { sesi, hapusSesi, type Peran } from '$lib/sesi';

	let { data, children }: {
		data: { peran: Peran; email: string; menu: { href: string; nama: string }[] };
		children: import('svelte').Snippet;
	} = $props();

	async function keluar() {
		await supabase().auth.signOut();
		hapusSesi();
		await goto('/masuk');
	}

	onMount(() => {
		// Sinkronkan sesi Supabase dengan store saat halaman dibuka.
		supabase()
			.auth.getSession()
			.then(({ data: s }) => {
				if (!s.session) goto('/masuk');
			});
	});
</script>

<div class="shell">
	<aside>
		<a class="logo" href="/app"><b>Focus</b><span>Management Nusantara</span></a>
		<nav>
			{#each data.menu as m}
				<a href={m.href}>{m.nama}</a>
			{/each}
		</nav>
		<div class="bawah">
			<span class="siapa">{data.email} - {data.peran}</span>
			<button on:click={keluar}>Keluar</button>
		</div>
	</aside>
	<main>
		{@render children()}
	</main>
</div>

<style>
	.shell {
		display: grid;
		grid-template-columns: 250px 1fr;
		min-height: 100vh;
	}
	aside {
		background: var(--ink-800);
		color: #dbe4ef;
		padding: 24px 18px;
		display: flex;
		flex-direction: column;
		gap: 22px;
		position: sticky;
		top: 0;
		height: 100vh;
	}
	.logo {
		display: flex;
		flex-direction: column;
		line-height: 1.1;
		color: #fff;
	}
	.logo b {
		font: 800 18px/1 var(--font-display);
	}
	.logo span {
		font-size: 10.5px;
		color: var(--text-300);
		letter-spacing: 0.12em;
		text-transform: uppercase;
	}
	nav {
		display: flex;
		flex-direction: column;
		gap: 4px;
	}
	nav a {
		padding: 10px 12px;
		border-radius: var(--r-sm);
		font-size: 14px;
	}
	nav a:hover {
		background: var(--ink-700);
		color: #fff;
	}
	.bawah {
		margin-top: auto;
		display: grid;
		gap: 8px;
		font-size: 13px;
		color: var(--text-400);
	}
	.bawah button {
		text-align: left;
		color: #fff;
		border: 1px solid var(--line-dark);
		border-radius: var(--r-sm);
		padding: 8px 12px;
	}
	main {
		padding: 28px;
		max-width: 1100px;
	}
	@media (max-width: 820px) {
		.shell {
			grid-template-columns: 1fr;
		}
		aside {
			position: static;
			height: auto;
		}
		main {
			padding: 18px var(--gutter) 40px;
		}
	}
</style>

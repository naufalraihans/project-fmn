<script lang="ts">
	import '../app.css';
	import { onMount } from 'svelte';
	import { muatSesi } from '$lib/sesi';

	onMount(() => {
		muatSesi();
	});

	let { children } = $props();
</script>

{@render children()}

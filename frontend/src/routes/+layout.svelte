<script lang="ts">
	import { onMount } from 'svelte';
	import '../styles.css';

	onMount(async () => {
		const { registerSW } = await import('virtual:pwa-register');
		registerSW({ immediate: true });
	});
</script>

<slot />

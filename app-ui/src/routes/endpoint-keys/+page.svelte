<script lang="ts">
	// Endpoint & Key (docs/SPEC-UI/001-SPEC-UI.md §6.2): one screen, one subject, the keys a client
	// presents to the gateway. Provider connections live on the provider screen.
	import GatewayKeysTab from '$lib/components/GatewayKeysTab.svelte';
</script>

<section class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Endpoint &amp; Key</h1>
		<p class="text-sm text-[var(--color-text-muted)]">Keys a client presents to the gateway.</p>
	</div>

	<GatewayKeysTab />
</section>

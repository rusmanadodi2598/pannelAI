<script lang="ts">
	// Endpoint & Key (docs/SPEC-UI/001-SPEC-UI.md §6.2).
	//
	// One screen, one subject: the keys a CLI tool presents to the gateway. Provider connections live on
	// the provider's own screen, which is where the reference keeps them (its Connections card), so this
	// screen carries no connections tab and nothing is duplicated across the two.
	import GatewayKeysTab from '$lib/components/GatewayKeysTab.svelte';
</script>

<section class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Endpoint &amp; Key</h1>
		<p class="text-sm text-[var(--color-text-muted)]">Keys a client presents to the gateway.</p>
	</div>

	<GatewayKeysTab />
</section>

<script lang="ts">
	// Endpoint & Key (docs/SPEC-UI/001-SPEC-UI.md §6.2).
	//
	// One screen, one subject: the keys a CLI tool presents to the gateway. The provider connections this
	// page used to carry as a second tab live on the provider's own screen, which is where the reference
	// keeps them (its Connections card), so the tab and its table moved there on 2026-09-24 rather than
	// being duplicated here.
	import GatewayKeysTab from '$lib/components/GatewayKeysTab.svelte';
</script>

<section class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Endpoint &amp; Key</h1>
		<p class="text-sm text-[var(--color-text-muted)]">Keys a CLI tool presents to the gateway.</p>
	</div>

	<GatewayKeysTab />
</section>

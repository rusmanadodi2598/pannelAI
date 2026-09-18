<script lang="ts">
	// Endpoint & Key (docs/SPEC-UI/001-SPEC-UI.md §6.2).
	//
	// The screen is a host: it owns the title and the tablist, and each tab owns its own data, states, and
	// dialogs. That split is what keeps the two tabs independent of each other, and it keeps every file
	// under the project line limit. Switching tabs remounts the one being shown, so each tab loads its own
	// data when it appears.
	import GatewayKeysTab from '$lib/components/GatewayKeysTab.svelte';
	import PanelTabs from '$lib/components/PanelTabs.svelte';
	import UpstreamEndpointsTab from '$lib/components/UpstreamEndpointsTab.svelte';

	const TABS = [
		{ id: 'gateway-keys', label: 'Gateway keys' },
		{ id: 'upstream', label: 'Upstream endpoints' }
	];
</script>

<section class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Endpoint &amp; Key</h1>
		<p class="text-sm text-[var(--color-text-muted)]">
			Keys a CLI tool presents to the gateway, and the provider connections the gateway routes to.
		</p>
	</div>

	<PanelTabs tabs={TABS} label="Endpoint and key views">
		{#snippet panel(active)}
			{#if active === 'upstream'}
				<UpstreamEndpointsTab />
			{:else}
				<GatewayKeysTab />
			{/if}
		{/snippet}
	</PanelTabs>
</section>

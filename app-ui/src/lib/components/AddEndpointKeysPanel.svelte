<script lang="ts">
	// The two ways §6.2 asks for adding keys to one endpoint: one form, and the repeatable row mode for
	// several keys in one submit loop.
	//
	// The mode lives here rather than in the drawer, which coordinates the screen's parts and would
	// otherwise carry a control that says nothing about the endpoint itself. A tablist is the panel's
	// control for "one of these views", and it brings arrow-key operation with it (R-32).
	import AddEndpointKeyForm from '$lib/components/AddEndpointKeyForm.svelte';
	import BulkAddKeysForm from '$lib/components/BulkAddKeysForm.svelte';
	import PanelTabs from '$lib/components/PanelTabs.svelte';

	let { endpointId, onadded }: { endpointId: string; onadded: () => void } = $props();

	const TABS = [
		{ id: 'one', label: 'One key' },
		{ id: 'many', label: 'Several keys' }
	];
</script>

<PanelTabs tabs={TABS} label="Add a key">
	{#snippet panel(active)}
		{#if active === 'many'}
			<BulkAddKeysForm {endpointId} {onadded} />
		{:else}
			<AddEndpointKeyForm {endpointId} {onadded} />
		{/if}
	{/snippet}
</PanelTabs>

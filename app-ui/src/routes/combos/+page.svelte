<script lang="ts">
	// Combos and the vision adapter (docs/SPEC-UI/001-SPEC-UI.md §6.4).
	//
	// The two tabs are one screen because they answer the same question from two sides: which model string
	// resolves to what. A combo is a client-chosen name over several upstream models; the vision adapter is a
	// router-side substitution the client never names. Both are the router's own configuration, which is why
	// §5.1 gives them one route.
	import PanelTabs from '$lib/components/PanelTabs.svelte';
	import CombosTab from '$lib/components/CombosTab.svelte';
	import VisionAdapterForm from '$lib/components/VisionAdapterForm.svelte';

	const TABS = [
		{ id: 'combos', label: 'Combos' },
		{ id: 'vision-adapter', label: 'Vision Adapter' }
	];
</script>

<section class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Combo & Vision Adapter</h1>
		<p class="text-sm text-[var(--color-text-muted)]">
			How a model string resolves to more than one upstream model, and what the router substitutes
			when a model cannot read an image.
		</p>
	</div>

	<PanelTabs tabs={TABS} label="Combo and vision adapter views">
		{#snippet panel(id)}
			{#if id === 'combos'}
				<CombosTab />
			{:else if id === 'vision-adapter'}
				<VisionAdapterForm />
			{/if}
		{/snippet}
	</PanelTabs>
</section>

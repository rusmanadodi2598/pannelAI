<script lang="ts">
	// The Models tab of the provider detail screen (docs/SPEC-UI/001-SPEC-UI.md §6.3): the reasoning picker
	// first, then the sets it applies to. The route carries both shapes of the screen, and the three stores
	// these sections read together are more than one file may hold.
	//
	// One probe store for the tab: a node's declared rows and a registry provider's catalog rows are the same
	// models addressed the same way, so they carry one verdict rather than one per table.
	//
	// The disabled set is global, so it is read once rather than per provider. A node does not read it: the
	// set narrows the registry catalog, and a node's screen has no catalog to narrow.
	//
	// The reasoning mode is this tab's own setting (SPEC-API §7.14, §7.15): the picker writes it and the model
	// tables read it for the `(level)` suffix they copy. It is read for every provider, including one whose
	// picker does not render: a node's rows carry levels too, and the tables still append the suffix.
	import { untrack } from 'svelte';
	import ModelCatalogList from '$lib/components/ModelCatalogList.svelte';
	import ProviderCustomModels from '$lib/components/ProviderCustomModels.svelte';
	import ProviderDisabledModels from '$lib/components/ProviderDisabledModels.svelte';
	import ProviderThinkingControl from '$lib/components/ProviderThinkingControl.svelte';
	import SectionFold from '$lib/components/SectionFold.svelte';
	import { createModelDisabledStore } from '$lib/stores/model-disabled.svelte';
	import { createModelTestStore } from '$lib/stores/model-test.svelte';
	import { createProviderThinkingStore } from '$lib/stores/provider-thinking.svelte';
	import type { ProviderDetail } from '$lib/schemas/provider';

	let {
		provider,
		providerId,
		custom,
		nodePrefix = ''
	}: {
		provider: ProviderDetail;
		providerId: string;
		custom: boolean;
		/** Reported by the node's own card: the prefix its model rows are addressed with. */
		nodePrefix?: string;
	} = $props();

	const modelTests = createModelTestStore();
	const disabled = createModelDisabledStore();
	const thinking = createProviderThinkingStore();
	let catalogToken = $state(0);

	// Reloads with the provider, like the route's own read: the store answers a mode only for the provider it
	// was last asked about, so a route swap shows no suffix until the new answer lands.
	$effect(() => {
		const id = providerId;
		untrack(() => void thinking.load(id));
	});

	$effect(() => {
		if (custom) return;
		untrack(() => void disabled.load());
	});

	function bumpCatalog(): void {
		catalogToken += 1;
	}
</script>

{#if custom}
	<SectionFold title="Available Models">
		<div class="flex flex-col gap-3">
			<ProviderThinkingControl {providerId} levels={provider.thinking_levels} {thinking} />
			<ProviderCustomModels
				{providerId}
				{thinking}
				tests={modelTests}
				{nodePrefix}
				onchanged={bumpCatalog}
			/>
		</div>
	</SectionFold>
{:else}
	<SectionFold title="Model catalog">
		<div class="flex flex-col gap-3">
			<ProviderThinkingControl
				providerId={provider.id}
				levels={provider.thinking_levels}
				{thinking}
			/>
			<ModelCatalogList
				tests={modelTests}
				{providerId}
				{disabled}
				{thinking}
				onchanged={bumpCatalog}
				token={catalogToken}
			/>
		</div>
	</SectionFold>

	<SectionFold title="Models this provider cannot route">
		<ProviderDisabledModels {providerId} {disabled} onchanged={bumpCatalog} />
	</SectionFold>

	<SectionFold title="Custom models">
		<ProviderCustomModels
			providerId={provider.id}
			{thinking}
			tests={modelTests}
			onchanged={bumpCatalog}
		/>
	</SectionFold>
{/if}

<script lang="ts">
	// The provider detail model catalog (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The component owns its query, so the provider is the only thing it must be told. Three parameters
	// reach the API: the provider scope, a capability filter, and free text. §6.3 also asks for a
	// "suggested" toggle, which is absent because the registry reports every model as suggested, so the
	// control would filter nothing (R-26).
	//
	// The rows and their Disable action are the table's business (ModelCatalogTable). This component
	// fetches, filters, and reports the states a fetch can be in.
	import { untrack } from 'svelte';
	import ModelCatalogTable from '$lib/components/ModelCatalogTable.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { listModelCatalog } from '$lib/api/models';
	import { CONTROL_ICONS } from '$lib/icons';
	import {
		CATALOG_CAPABILITY_FILTERS,
		catalogQueryParams,
		type CatalogModel
	} from '$lib/schemas/model';
	import type { ModelDisabledStore } from '$lib/stores/model-disabled.svelte';
	import type { ProviderThinkingStore } from '$lib/stores/provider-thinking.svelte';

	let {
		providerId,
		disabled,
		thinking,
		onchanged,
		token = 0
	}: {
		providerId: string;
		disabled: ModelDisabledStore;
		thinking: ProviderThinkingStore;
		onchanged: () => void;
		// Bumped by the page when a write elsewhere changes this list: disabling a model removes its row,
		// enabling one brings it back, and a custom model joins it.
		token?: number;
	} = $props();

	const SearchIcon = CONTROL_ICONS.search.icon;
	const ClearIcon = CONTROL_ICONS.clear.icon;

	let models = $state<CatalogModel[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);

	// `draft` is what the operator types; `applied` is what the API was asked. Keeping them apart is what
	// lets the search box hold a half-typed word without firing a request per keystroke.
	let draft = $state('');
	let applied = $state('');
	let capability = $state('');

	const filtered = $derived(applied !== '' || capability !== '');

	// Reloads when the route's provider changes and when the token moves. The filter state is read
	// untracked because the handlers below already reload on a filter change, and the reads inside `load`
	// happen synchronously before its first await: tracking them here would fire a second, identical
	// request on every filter click.
	$effect(() => {
		const provider = providerId;
		const revision = token;
		untrack(() => void load(provider, revision));
	});

	async function load(provider: string, revision: number): Promise<void> {
		loading = true;
		const result = await listModelCatalog(
			catalogQueryParams({ providerId: provider, capability, query: applied })
		);

		// A newer load owns this state now, so an older answer that lands late is dropped rather than
		// rendering a list the operator has already moved past.
		if (revision !== token) return;

		loading = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		models = result.data.data;
	}

	function applySearch(): void {
		applied = draft;
		void load(providerId, token);
	}

	function toggleCapability(value: string): void {
		capability = capability === value ? '' : value;
		void load(providerId, token);
	}

	function clearFilters(): void {
		draft = '';
		applied = '';
		capability = '';
		void load(providerId, token);
	}
</script>

<div class="flex flex-col gap-4">
	<div class="flex flex-col gap-3">
		<form
			class="flex flex-wrap items-end gap-3"
			onsubmit={(event) => {
				event.preventDefault();
				applySearch();
			}}
		>
			<label class="flex flex-col gap-1 text-sm">
				<span class="text-[var(--color-text-muted)]">Search models</span>
				<input
					type="search"
					bind:value={draft}
					placeholder="Model id or name"
					class="min-h-11 w-64 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
				/>
			</label>
			<button type="submit" class="inline-flex min-h-11 items-center gap-2 underline">
				<SearchIcon class="size-4" aria-hidden="true" />
				Search
			</button>
		</form>

		<div class="flex flex-wrap items-center gap-2">
			<span class="text-sm text-[var(--color-text-muted)]">Capability</span>
			{#each CATALOG_CAPABILITY_FILTERS as option (option)}
				<button
					type="button"
					aria-pressed={capability === option}
					class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm aria-pressed:bg-[var(--color-surface-2)]"
					onclick={() => toggleCapability(option)}>{option}</button
				>
			{/each}
			{#if filtered}
				<button
					type="button"
					class="inline-flex min-h-11 items-center gap-2 underline"
					onclick={clearFilters}
				>
					<ClearIcon class="size-4" aria-hidden="true" />
					Clear filters
				</button>
			{/if}
		</div>
	</div>

	{#if loading}
		<StateMessage kind="loading" title="Loading the model catalog" />
	{:else if error}
		<StateMessage kind="error" title="The model catalog could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => load(providerId, token)}
					>Try again</button
				>
			{/snippet}
		</StateMessage>
	{:else if models.length === 0 && filtered}
		<StateMessage
			kind="empty"
			title="No models found for this filter"
			description="Clear the filters to see every model this provider offers."
		>
			{#snippet action()}
				<button type="button" class="underline" onclick={clearFilters}>Clear filters</button>
			{/snippet}
		</StateMessage>
	{:else if models.length === 0}
		<StateMessage
			kind="empty"
			title="This provider offers no models"
			description="The catalog reports nothing for this provider. A model the gateway has disabled is left out of the catalog, so check the disabled set before treating this as an empty registry entry."
		/>
	{:else}
		<ModelCatalogTable {models} {disabled} {thinking} {onchanged} />

		<p class="text-sm text-[var(--color-text-muted)]">
			{models.length}
			{models.length === 1 ? 'model' : 'models'}
			{#if filtered}match these filters{:else}in this provider{/if}.
		</p>
	{/if}
</div>

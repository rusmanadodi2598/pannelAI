<script lang="ts">
	// Provider registry list (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// Read-only over the registry: the registry is embedded and the API owns it, so this screen answers
	// what the gateway can route to and whether it has a working account, and the registry half writes
	// nothing. The category filter and the search are the two the API accepts: the search reads
	// `?q`, which the server matches over id and display name (PORT 002 D1), because §6.3's pagination
	// discipline forbids pulling the registry into a client-side filter.
	//
	// The custom provider controls create a node, and the gateway synthesizes it into the registry, so the
	// table is the one place a custom provider appears. The page reads only the registry; a separate node
	// read would duplicate every custom row. Custom rows carry a Custom marker in the table.
	import CustomProviderSection from '$lib/components/CustomProviderSection.svelte';
	import ProvidersFilters from '$lib/components/ProvidersFilters.svelte';
	import PagingControls from '$lib/components/PagingControls.svelte';
	import ProviderTable from '$lib/components/ProviderTable.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import { listProviders } from '$lib/api/providers';
	import type { ApiError } from '$lib/api/errors';
	import { schemaProviderQuery, type Provider } from '$lib/schemas/provider';
	import { onMount } from 'svelte';

	const PAGE_SIZE = 25;

	let providers = $state<Provider[]>([]);
	let total = $state(0);
	let page = $state(1);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);
	let category = $state('');

	// `draft` is what the operator types; `applied` is what the API was asked. The pair keeps a half-typed
	// word from firing a request per keystroke, the same contract the model catalog's search holds.
	let searchDraft = $state('');
	let searchApplied = $state('');
	let searchError = $state<string | null>(null);

	const lastPage = $derived(Math.max(1, Math.ceil(total / PAGE_SIZE)));
	const filtered = $derived(category !== '' || searchApplied !== '');

	onMount(() => void loadProviders());

	async function loadProviders(): Promise<void> {
		loading = true;
		const result = await listProviders({
			page,
			per_page: PAGE_SIZE,
			category: category === '' ? undefined : category,
			q: searchApplied === '' ? undefined : searchApplied
		});
		loading = false;

		if (!result.ok) {
			error = result.error;
			return;
		}

		error = null;
		providers = result.data.data;
		total = result.data.meta.total;
	}

	function applyCategory(value: string): void {
		category = value;
		page = 1;
		void loadProviders();
	}

	// The typed value is trimmed before it is asked for, so a stray space does not become a filter the
	// operator cannot see, and a filter change always restarts the paging.
	function applySearch(): void {
		const search = searchDraft.trim();
		const validated = schemaProviderQuery.safeParse({ q: search === '' ? undefined : search });
		if (!validated.success) {
			searchError = 'Search must be 120 characters or fewer.';
			return;
		}

		searchError = null;
		searchApplied = validated.data.q ?? '';
		page = 1;
		void loadProviders();
	}

	function goPage(next: number): void {
		page = next;
		void loadProviders();
	}

	function clearFilters(): void {
		category = '';
		searchDraft = '';
		searchApplied = '';
		searchError = null;
		page = 1;
		void loadProviders();
	}
</script>

<section class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Provider</h1>
		<p class="text-sm text-[var(--color-text-muted)]">
			What the gateway can route, and each account's status.
		</p>
	</div>

	<ProvidersFilters
		bind:draft={searchDraft}
		{category}
		{searchError}
		oncategory={applyCategory}
		onsearch={applySearch}
		onrefresh={loadProviders}
	/>

	<CustomProviderSection onsaved={loadProviders} />

	{#if loading}
		<StateMessage kind="loading" title="Loading the provider registry" />
	{:else if error}
		<StateMessage kind="error" title="Could not load the provider registry" apiError={error}>
			{#snippet action()}
				<Button variant="ghost" class="underline" onclick={loadProviders}>Try again</Button>
			{/snippet}
		</StateMessage>
	{:else if providers.length === 0}
		<StateMessage
			kind="empty"
			title={filtered ? 'No provider matches this filter' : 'The registry is empty'}
			description={filtered
				? 'Filters narrow the registry. Clear both to see every provider.'
				: 'No providers registered yet. Nothing to route to.'}
		>
			{#snippet action()}
				{#if filtered}
					<Button variant="ghost" class="underline" onclick={clearFilters}>
						Clear the filters
					</Button>
				{/if}
			{/snippet}
		</StateMessage>
	{:else}
		<ProviderTable {providers} />

		<PagingControls {page} {lastPage} onpage={goPage} />
	{/if}
</section>

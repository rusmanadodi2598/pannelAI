<script lang="ts">
	// Provider registry list (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// Read-only over the registry: the registry is embedded and the API owns it, so this screen answers
	// what the gateway can route to and whether it has a working account, and the registry half writes
	// nothing. The category filter and the search are the two the API accepts: the search reads
	// `?q`, which the server matches over id and display name (PORT 002 D1), because §6.3's pagination
	// discipline forbids pulling the registry into a client-side filter.
	//
	// The custom provider section is the exception, and it is why this page owns a second read: a node is
	// operator-created, so the section writes. Its set is not paginated and not narrowed by either
	// filter, which is why it is read on its own rather than derived from the registry page.
	import CustomProviderSection from '$lib/components/CustomProviderSection.svelte';
	import ProvidersFilters from '$lib/components/ProvidersFilters.svelte';
	import PagingControls from '$lib/components/PagingControls.svelte';
	import ProviderTable from '$lib/components/ProviderTable.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import { listProviders } from '$lib/api/providers';
	import { listProviderNodes } from '$lib/api/provider-nodes';
	import type { ApiError } from '$lib/api/errors';
	import { schemaProviderQuery, type Provider } from '$lib/schemas/provider';
	import type { ProviderNode } from '$lib/schemas/provider-node';
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

	let nodes = $state<ProviderNode[]>([]);
	let nodesLoading = $state(true);
	let nodesError = $state<ApiError | null>(null);

	const lastPage = $derived(Math.max(1, Math.ceil(total / PAGE_SIZE)));
	const filtered = $derived(category !== '' || searchApplied !== '');

	onMount(() => void load());

	// One entry point for both reads, so the screen's single refresh control re-reads everything on it
	// (§8.6.2) rather than the registry alone.
	async function load(): Promise<void> {
		await Promise.all([loadProviders(), loadNodes()]);
	}

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

	async function loadNodes(): Promise<void> {
		nodesLoading = true;
		const result = await listProviderNodes();
		nodesLoading = false;

		if (!result.ok) {
			nodesError = result.error;
			return;
		}

		nodesError = null;
		nodes = result.data.data;
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
		onrefresh={load}
	/>

	<CustomProviderSection {nodes} loading={nodesLoading} error={nodesError} onreload={loadNodes} />

	{#if loading}
		<StateMessage kind="loading" title="Loading the provider registry" />
	{:else if error}
		<StateMessage kind="error" title="Could not load the provider registry" apiError={error}>
			{#snippet action()}
				<Button variant="ghost" class="underline" onclick={load}>Try again</Button>
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

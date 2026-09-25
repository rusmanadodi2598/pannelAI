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
	import ProviderTable from '$lib/components/ProviderTable.svelte';
	import RefreshControl from '$lib/components/RefreshControl.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { CONTROL_ICONS } from '$lib/icons';
	import { listProviders } from '$lib/api/providers';
	import { listProviderNodes } from '$lib/api/provider-nodes';
	import { PROVIDER_CATEGORIES, type Provider } from '$lib/schemas/provider';
	import type { ProviderNode } from '$lib/schemas/provider-node';
	import { onMount } from 'svelte';

	const PAGE_SIZE = 25;

	const SearchIcon = CONTROL_ICONS.search.icon;
	const PreviousIcon = CONTROL_ICONS.previous.icon;
	const NextIcon = CONTROL_ICONS.next.icon;

	let providers = $state<Provider[]>([]);
	let total = $state(0);
	let page = $state(1);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let category = $state('');

	// `draft` is what the operator types; `applied` is what the API was asked. The pair keeps a half-typed
	// word from firing a request per keystroke, the same contract the model catalog's search holds.
	let searchDraft = $state('');
	let searchApplied = $state('');

	let nodes = $state<ProviderNode[]>([]);
	let nodesLoading = $state(true);
	let nodesError = $state<string | null>(null);

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
			error = result.error.message;
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
			nodesError = result.error.message;
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
		searchApplied = searchDraft.trim();
		page = 1;
		void loadProviders();
	}

	function clearFilters(): void {
		category = '';
		searchDraft = '';
		searchApplied = '';
		page = 1;
		void loadProviders();
	}
</script>

<section class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Provider</h1>
		<p class="text-sm text-[var(--color-text-muted)]">
			What the gateway can route to, and how the stored accounts for each provider are doing.
		</p>
	</div>

	<!-- One compact row for the three controls the operator reaches for together (owner directive,
	     2026-09-25): the search commits on submit, the select commits on change, and the refresh
	     re-reads both this list and the node set. Every control shares one height and one baseline, and
	     the row wraps as a block on a narrow screen rather than stretching any single control. -->
	<div class="flex flex-wrap items-stretch gap-2">
		<form
			class="flex min-w-0 flex-1 items-stretch gap-2"
			onsubmit={(event) => {
				event.preventDefault();
				applySearch();
			}}
		>
			<input
				type="search"
				bind:value={searchDraft}
				placeholder="Search name or id"
				aria-label="Search providers by name or id"
				class="min-h-11 min-w-0 flex-1 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
			/>
			<button
				type="submit"
				class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
			>
				<SearchIcon class="size-4" aria-hidden="true" />
				Search
			</button>
		</form>

		<select
			value={category}
			onchange={(event) => applyCategory(event.currentTarget.value)}
			aria-label="Category"
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
		>
			<option value="">All categories</option>
			{#each PROVIDER_CATEGORIES as option (option)}
				<option value={option}>{option}</option>
			{/each}
		</select>

		<RefreshControl onrefresh={load} />
	</div>

	<CustomProviderSection {nodes} loading={nodesLoading} error={nodesError} onreload={loadNodes} />

	{#if loading}
		<StateMessage kind="loading" title="Loading the provider registry" />
	{:else if error}
		<StateMessage
			kind="error"
			title="The provider registry could not be loaded"
			description={error}
		>
			{#snippet action()}
				<button type="button" class="underline" onclick={load}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if providers.length === 0}
		<StateMessage
			kind="empty"
			title={filtered ? 'No provider matches this filter' : 'The registry is empty'}
			description={filtered
				? 'The category filter and the search narrow the same registry. Clearing both shows every provider again.'
				: 'The embedded registry reported no providers, which means the service was built without one.'}
		>
			{#snippet action()}
				{#if filtered}
					<button type="button" class="underline" onclick={clearFilters}>Clear the filters</button>
				{/if}
			{/snippet}
		</StateMessage>
	{:else}
		<ProviderTable {providers} />

		<div class="flex items-center gap-3 text-sm">
			<button
				type="button"
				class="inline-flex min-h-11 items-center gap-1 underline disabled:opacity-50"
				disabled={page <= 1}
				onclick={() => {
					page -= 1;
					void loadProviders();
				}}
			>
				<PreviousIcon class="size-4" aria-hidden="true" />
				Previous
			</button>
			<span class="text-[var(--color-text-muted)]">Page {page} of {lastPage}</span>
			<button
				type="button"
				class="inline-flex min-h-11 items-center gap-1 underline disabled:opacity-50"
				disabled={page >= lastPage}
				onclick={() => {
					page += 1;
					void loadProviders();
				}}
			>
				Next
				<NextIcon class="size-4" aria-hidden="true" />
			</button>
		</div>
	{/if}
</section>

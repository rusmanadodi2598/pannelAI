<script lang="ts">
	// Upstream endpoints tab of Endpoint & Key (docs/SPEC-UI/001-SPEC-UI.md §6.2, tab 2).
	//
	// The tab owns the URL-backed filters, the create form's visibility, and the data states; the table and
	// the create form are their own components.
	//
	// Every filter is URL state and the request is built from the URL (§8.4.2), so the table is never
	// narrowed in the browser: one page of endpoints is not the data set, and §8.4.1 forbids filtering it
	// locally. The API reads `provider_id` and `status` itself (SPEC-API §7.5), so the panel sends what the
	// operator picked rather than filtering what came back.
	//
	// Two provider-shaped inputs live here and they are not the same thing: `provider` is §6.3's arrival
	// parameter and decides which provider the create form targets, while `provider_id` is the table's
	// filter. The filter's options come from a second, unfiltered read of the same route, because deriving
	// them from the filtered result would strand the operator on the provider they picked: every other
	// provider would leave the select until the filter was cleared.
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import CreateEndpointForm from '$lib/components/CreateEndpointForm.svelte';
	import EndpointDetailDrawer from '$lib/components/EndpointDetailDrawer.svelte';
	import EndpointFilters from '$lib/components/EndpointFilters.svelte';
	import EndpointTable from '$lib/components/EndpointTable.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { listEndpoints } from '$lib/api/endpoints';
	import type { Endpoint } from '$lib/schemas/endpoint';
	import {
		OPTION_PAGE_SIZE,
		toProviderOptions,
		withSelectedProvider,
		type ProviderOption
	} from '$lib/schemas/endpoint-options';
	import {
		ENDPOINTS_PAGE_SIZE,
		endpointFiltersApplied,
		nextEndpointSearch,
		parseEndpointSearch,
		type EndpointSearch
	} from '$lib/schemas/endpoint-search';

	let endpoints = $state<Endpoint[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let error = $state<string | null>(null);

	let providerOptions = $state<ProviderOption[]>([]);
	let optionsNotice = $state<string | null>(null);

	let selected = $state<Endpoint | null>(null);
	let creating = $state(true);

	const search = $derived(parseEndpointSearch(page.url.searchParams));
	const chosenProvider = $derived(page.url.searchParams.get('provider') ?? '');
	const lastPage = $derived(Math.max(1, Math.ceil(total / ENDPOINTS_PAGE_SIZE)));
	const filtered = $derived(endpointFiltersApplied(search));

	// The API orders by priority already, but the panel does not assume it: a stable order is what makes the
	// priority column readable, so it sorts by priority and then by label.
	const ordered = $derived(
		[...endpoints].sort(
			(left, right) => left.priority - right.priority || left.label.localeCompare(right.label)
		)
	);

	// The provider the URL names is always offered, even when no read returned it: a select whose value has
	// no matching option renders blank, which would hide the filter the operator is looking at.
	const options = $derived(withSelectedProvider(providerOptions, search.providerId));

	// Reloads whenever the URL changes, which is every filter and every page on this tab.
	$effect(() => {
		const filters = search;
		untrack(() => void load(filters));
	});

	async function load(filters: EndpointSearch): Promise<void> {
		loading = true;
		const [pageResult, optionsResult] = await Promise.all([
			listEndpoints({
				provider_id: filters.providerId,
				status: filters.status,
				page: filters.page,
				per_page: ENDPOINTS_PAGE_SIZE
			}),
			// Unfiltered and at the API's page cap: this read feeds the filter's option list, not the table.
			listEndpoints({ page: 1, per_page: OPTION_PAGE_SIZE })
		]);
		loading = false;

		// The option read is best effort: when it fails the filter still offers what this result carries,
		// and the notice says why the list is short rather than presenting it as the whole registry.
		if (optionsResult.ok) {
			optionsNotice = null;
			providerOptions = toProviderOptions(optionsResult.data.data);
		} else {
			optionsNotice = `The provider list could not be read (${optionsResult.error.message}), so the filter offers only the providers in this result.`;
			if (pageResult.ok) providerOptions = toProviderOptions(pageResult.data.data);
		}

		if (!pageResult.ok) {
			error = pageResult.error.message;
			return;
		}

		error = null;
		endpoints = pageResult.data.data;
		total = pageResult.data.meta.total;
	}

	function navigate(params: URLSearchParams): void {
		const queryString = params.toString();
		void goto(resolve(queryString === '' ? '/endpoint-keys' : `/endpoint-keys?${queryString}`), {
			keepFocus: true
		});
	}

	function change(key: string, value: string): void {
		navigate(nextEndpointSearch(page.url.searchParams, key, value));
	}

	function clearFilters(): void {
		let next = page.url.searchParams;
		for (const key of ['provider_id', 'status']) next = nextEndpointSearch(next, key, '');
		navigate(next);
	}
</script>

<div class="flex flex-col gap-5">
	{#if chosenProvider && creating}
		<div class="flex flex-col gap-2">
			<CreateEndpointForm
				providerId={chosenProvider}
				oncreated={() => {
					creating = false;
					void load(search);
				}}
			/>
			<button type="button" class="min-h-11 self-start underline" onclick={() => (creating = false)}
				>Hide this form</button
			>
		</div>
	{/if}

	<EndpointFilters {search} {options} {optionsNotice} onchange={change} onclear={clearFilters} />

	{#if loading}
		<StateMessage kind="loading" title="Loading upstream endpoints" />
	{:else if error}
		<StateMessage kind="error" title="Upstream endpoints could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => load(search)}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if endpoints.length === 0 && filtered}
		<StateMessage
			kind="empty"
			title="No endpoint matches these filters"
			description="Clear the filters to see the whole list again."
		>
			{#snippet action()}
				<button type="button" class="underline" onclick={clearFilters}>Clear filters</button>
			{/snippet}
		</StateMessage>
	{:else if endpoints.length === 0 && search.page > 1 && total > 0}
		<StateMessage
			kind="empty"
			title="Nothing on this page"
			description={`The list holds ${total} endpoints, so this page is past the end of it.`}
		>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => change('page', '')}
					>Go to the first page</button
				>
			{/snippet}
		</StateMessage>
	{:else if endpoints.length === 0}
		<StateMessage
			kind="empty"
			title="No upstream endpoints yet"
			description="Pick a provider, then add an endpoint for it to start routing."
		>
			{#snippet action()}
				<a href={resolve('/providers')} class="underline">Choose a provider</a>
			{/snippet}
		</StateMessage>
	{:else}
		<EndpointTable endpoints={ordered} onopen={(entry) => (selected = entry)} />

		<div class="flex items-center gap-3 text-sm">
			<button
				type="button"
				class="underline disabled:opacity-50"
				disabled={search.page <= 1}
				onclick={() => change('page', String(search.page - 1))}>Previous</button
			>
			<span class="text-[var(--color-text-muted)]">Page {search.page} of {lastPage}</span>
			<button
				type="button"
				class="underline disabled:opacity-50"
				disabled={search.page >= lastPage}
				onclick={() => change('page', String(search.page + 1))}>Next</button
			>
		</div>
	{/if}
</div>

<EndpointDetailDrawer
	entry={selected}
	onclose={() => (selected = null)}
	onchanged={() => load(search)}
/>

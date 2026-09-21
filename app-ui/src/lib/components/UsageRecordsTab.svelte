<script lang="ts">
	// The Usage Records tab (docs/SPEC-UI/001-SPEC-UI.md §6.5, tab 2).
	//
	// Every filter is URL state (§8.4.2), and the URL is what the request is built from: the table is never
	// filtered in the browser, because §8.4.1 forbids it and one page of records is not the data set. The
	// filter bar owns its own drafts and this component owns the request, so neither has to know how the
	// other holds a half-typed value.
	//
	// The period selector is the "date range" filter §6.5 lists. A native date pair would take a local
	// calendar date against an API that reads UTC instants, and the periods are already defined at screen
	// level, so reusing the Overview's control is both simpler and less ambiguous.
	//
	// The empty state's link to API Docs is deferred until that route exists, because R-24 forbids a link to
	// a screen the panel does not have.
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import RefreshControl from '$lib/components/RefreshControl.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import UsageRecordDrawer from '$lib/components/UsageRecordDrawer.svelte';
	import UsageRecordTable from '$lib/components/UsageRecordTable.svelte';
	import UsageRecordsFilters from '$lib/components/UsageRecordsFilters.svelte';
	import { listUsageRecords } from '$lib/api/usage';
	import type { UsageRecord } from '$lib/schemas/usage';
	import {
		nextUsageSearch,
		parseUsageSearch,
		usageFiltersApplied,
		type UsageSearch
	} from '$lib/schemas/usage-search';
	import { periodRange, usageQuery, USAGE_RECORDS_PAGE_SIZE } from '$lib/schemas/usage-view';

	let records = $state<UsageRecord[]>([]);
	let total = $state(0);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let opened = $state<string | null>(null);

	const search = $derived(parseUsageSearch(page.url.searchParams));
	const lastPage = $derived(Math.max(1, Math.ceil(total / USAGE_RECORDS_PAGE_SIZE)));
	const filtered = $derived(usageFiltersApplied(search));

	// Reloads whenever the URL changes, which is every filter on this tab.
	$effect(() => {
		const filters = search;
		untrack(() => void load(filters));
	});

	async function load(filters: UsageSearch): Promise<void> {
		loading = true;
		const result = await listUsageRecords(
			usageQuery({
				...periodRange(filters.period, new Date()),
				status: filters.status,
				endpointId: filters.endpointId,
				model: filters.model,
				query: filters.query,
				page: filters.page
			})
		);
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		records = result.data.data;
		total = result.data.meta.total;
	}

	function navigate(params: URLSearchParams): void {
		const queryString = params.toString();
		void goto(resolve(queryString === '' ? '/usage' : `/usage?${queryString}`), {
			keepFocus: true
		});
	}

	function change(key: string, value: string): void {
		navigate(nextUsageSearch(page.url.searchParams, key, value));
	}

	function applyFilters(values: { endpointId: string; model: string; query: string }): void {
		const fields = [
			['endpoint_id', values.endpointId],
			['model', values.model],
			['q', values.query]
		] as const;

		let next = page.url.searchParams;
		for (const [key, value] of fields) next = nextUsageSearch(next, key, value.trim());

		navigate(next);
	}

	function clearFilters(): void {
		let next = page.url.searchParams;
		for (const key of ['status', 'endpoint_id', 'model', 'q'])
			next = nextUsageSearch(next, key, '');

		navigate(next);
	}
</script>

<div class="flex flex-col gap-5">
	<UsageRecordsFilters {search} onchange={change} onapply={applyFilters} onclear={clearFilters} />

	<RefreshControl onrefresh={() => load(search)} />

	{#if search.notices.length > 0}
		<div
			role="status"
			aria-live="polite"
			class="flex flex-col gap-1 rounded-[var(--radius-md)] bg-[var(--color-surface-2)] px-3 py-2 text-sm"
		>
			{#each search.notices as notice (notice)}
				<p>{notice}</p>
			{/each}
		</div>
	{/if}

	{#if loading}
		<StateMessage kind="loading" title="Loading requests" />
	{:else if error}
		<StateMessage kind="error" title="Requests could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => load(search)}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if records.length === 0 && filtered}
		<StateMessage
			kind="empty"
			title="No requests match these filters"
			description="The period may still hold records that these filters exclude. Clear the filters to see what it holds."
		>
			{#snippet action()}
				<button type="button" class="underline" onclick={clearFilters}>Clear filters</button>
			{/snippet}
		</StateMessage>
	{:else if records.length === 0}
		<StateMessage
			kind="empty"
			title="No requests in this window"
			description="Nothing was routed in the selected period. The gateway writes a record here once a client sends a request with a gateway key, so the likely cause is that no client has called it yet."
		>
			{#snippet action()}
				{#if search.period !== '60d'}
					<button type="button" class="underline" onclick={() => change('period', '60d')}
						>Look back 60 days</button
					>
				{/if}
			{/snippet}
		</StateMessage>
	{:else}
		<UsageRecordTable {records} onopen={(record) => (opened = record.request_id)} />

		<div class="flex items-center gap-3 text-sm">
			<button
				type="button"
				class="underline disabled:opacity-50"
				disabled={search.page <= 1}
				onclick={() => change('page', String(search.page - 1))}>Previous</button
			>
			<span class="text-[var(--color-text-muted)]">
				Page {search.page} of {lastPage}, {total}
				{total === 1 ? 'request' : 'requests'}
			</span>
			<button
				type="button"
				class="underline disabled:opacity-50"
				disabled={search.page >= lastPage}
				onclick={() => change('page', String(search.page + 1))}>Next</button
			>
		</div>
	{/if}
</div>

<UsageRecordDrawer requestId={opened} onclose={() => (opened = null)} />

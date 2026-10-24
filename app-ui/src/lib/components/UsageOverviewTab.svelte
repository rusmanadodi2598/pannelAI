<script lang="ts">
	// The Usage Overview tab (docs/SPEC-UI/001-SPEC-UI.md §6.5).
	//
	// The period and the group-by are URL state. §8.4.2 makes a filtered view shareable, and here it also
	// keeps the window honest: the URL carries the period *name* rather than the instants it resolves to, so
	// a shared link keeps meaning "the last 7 days" instead of freezing whichever hour it was read in. Each
	// read resolves the range from one captured now, which is what makes two reads of the same period
	// comparable. The breakdown's sort is URL state for the same reason, and it is applied to the whole
	// response rather than to a page of it.
	//
	// The screen computes no figure of its own. Every number here comes from the summary or the timeseries,
	// with one stated exception: the token series and the two bar charts add tokens in to tokens out, and
	// their captions say so. `latency_ms` is returned by the API and deliberately not shown, because it is a
	// sum of per-request latencies rather than a mean, and §6.5 asks for p50 and p95 rather than a total.
	//
	// The breakdown table is on screen from the first read: the API's group block arrives
	// with the same response, and "which models consumed this window" is the first question it answers.
	// Turning it off is an explicit choice the URL names.
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { Button } from '$lib/primitives/button/index.js';
	import { untrack } from 'svelte';
	import { SvelteURLSearchParams } from 'svelte/reactivity';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import type { ApiError } from '$lib/api/errors';
	import UsageLivePanel from '$lib/components/UsageLivePanel.svelte';
	import UsageOverviewBarCharts from '$lib/components/UsageOverviewBarCharts.svelte';
	import UsageOverviewBreakdown from '$lib/components/UsageOverviewBreakdown.svelte';
	import UsageOverviewCharts from '$lib/components/UsageOverviewCharts.svelte';
	import UsageOverviewControls from '$lib/components/UsageOverviewControls.svelte';
	import UsageOverviewEmpty from '$lib/components/UsageOverviewEmpty.svelte';
	import UsageTotalsTiles from '$lib/components/UsageTotalsTiles.svelte';
	import { listProviders } from '$lib/api/providers';
	import type { Provider } from '$lib/schemas/provider';
	import { getUsageSummary, getUsageTimeseries } from '$lib/api/usage';
	import { type UsageSort, type UsageSummary, type UsageTimeseries } from '$lib/schemas/usage';
	import { granularityFor, periodRange, sortGroups, usageQuery } from '$lib/schemas/usage-view';
	import { providerNameMap } from '$lib/schemas/usage-topology-nodeset';
	import { nextUsageSearch, parseUsageSearch, type UsageSearch } from '$lib/schemas/usage-search';

	let summary = $state<UsageSummary | null>(null);
	let timeseries = $state<UsageTimeseries | null>(null);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);
	// The provider registry is read once here and shared with the breakdown, the provider chart, and the
	// live drawing. A registry that cannot be read leaves the tables rendering ids and the drawing with no
	// nodes, with one line saying both.
	let providers = $state<Provider[]>([]);
	let providerNames = $state<Map<string, string> | null>(null);
	let providersNotice = $state<{ text: string; apiError?: ApiError } | null>(null);

	const search = $derived(parseUsageSearch(page.url.searchParams));

	// Reloads whenever the URL changes, which is every control on this tab: the handlers navigate rather
	// than load, so there is one path from a control to a request. The reads inside `load` happen before its
	// first await, so they are untracked to stop this effect from tracking its own inputs.
	$effect(() => {
		const filters = search;
		untrack(() => void load(filters));
	});

	async function load(filters: UsageSearch): Promise<void> {
		loading = true;
		const query = usageQuery({
			...periodRange(filters.period, new Date()),
			groupBy: filters.groupBy,
			granularity: granularityFor(filters.period)
		});

		const [summaryResult, seriesResult] = await Promise.all([
			getUsageSummary(query),
			getUsageTimeseries(query)
		]);
		loading = false;

		if (!summaryResult.ok) {
			error = summaryResult.error;
			return;
		}
		if (!seriesResult.ok) {
			error = seriesResult.error;
			return;
		}

		error = null;
		summary = summaryResult.data;
		timeseries = seriesResult.data;
	}

	const buckets = $derived(timeseries?.buckets ?? []);
	const totals = $derived(summary?.totals ?? null);
	// The sort is the panel's own: §7.12 has no sort parameter, and the API sends the whole breakdown for
	// the window, so the order is applied to every group rather than to a page of them.
	const groups = $derived(sortGroups(summary?.groups ?? [], search.sort, search.order));
	const windowReadout = $derived(
		summary && timeseries
			? { from: summary.from, to: summary.to, granularity: timeseries.granularity }
			: null
	);
	const recordPath = $derived(
		search.groupBy === 'provider' || search.groupBy === 'model' ? recordsPath : null
	);

	function navigate(params: URLSearchParams): void {
		const next = params.toString();
		void goto(resolve(next === '' ? '/usage' : `/usage?${next}`), { keepFocus: true });
	}

	function change(key: string, value: string): void {
		navigate(nextUsageSearch(page.url.searchParams, key, value));
	}

	/**
	 * The Records view behind one breakdown row, carrying the dimension the row was grouped by.
	 *
	 * The key the API grouped by is the raw provider id or model id, which is exactly what the records
	 * filters take. The page parameter is dropped because the matching rows start at the first page.
	 */
	function recordsPath(key: string): `/usage?${string}` {
		const next = new SvelteURLSearchParams(page.url.searchParams);
		next.set('tab', 'records');
		next.delete('page');

		if (search.groupBy === 'provider') {
			next.set('provider_id', key);
			next.delete('model');
		} else {
			next.set('model', key);
			next.delete('provider_id');
		}

		return `/usage?${next.toString()}`;
	}

	/**
	 * A header click: the same column flips the direction, a new column starts ascending, and clearing the
	 * sort drops both parameters.
	 */
	function sortBy(field: UsageSort | ''): void {
		if (field === '') {
			const cleared = nextUsageSearch(
				nextUsageSearch(page.url.searchParams, 'sort', ''),
				'order',
				''
			);
			navigate(cleared);
			return;
		}

		const order = search.sort === field && search.order === 'asc' ? 'desc' : 'asc';
		const next = nextUsageSearch(
			nextUsageSearch(page.url.searchParams, 'sort', field),
			'order',
			order
		);
		navigate(next);
	}

	async function loadProviders(): Promise<void> {
		const result = await listProviders({ per_page: 100 });

		if (!result.ok) {
			providers = [];
			providerNames = null;
			providersNotice = {
				text: 'Providers could not be read. The drawing has no nodes. Ids show instead. ',
				apiError: result.error
			};
			return;
		}

		const { data, meta } = result.data;
		providers = data;
		providerNames = providerNameMap(data);
		providersNotice =
			meta.total > data.length
				? { text: `Shows the first ${data.length} of ${meta.total} providers.` }
				: null;
	}

	function refresh(): void {
		void load(search);
		void loadProviders();
	}

	// The registry is read once on mount, because the provider chart and the live drawing are on screen
	// whatever the breakdown table is showing. The effect reads nothing reactive, so it runs when the tab
	// mounts and not on every load; a refresh repeats the read through its own call above.
	$effect(() => {
		void loadProviders();
	});
</script>

<div class="flex flex-col gap-5">
	<UsageOverviewControls {search} window={windowReadout} onchange={change} onrefresh={refresh} />

	<!-- The one live thing on the screen sits directly under the toolbar, outside the aggregate read's
	     branches: it is what an operator opens the tab for during an incident, and a failed or empty
	     aggregate read cannot take it off the screen. -->
	<UsageLivePanel {providers} {providersNotice} />

	{#if loading}
		<StateMessage kind="loading" title="Loading usage" />
	{:else if error}
		<StateMessage kind="error" title="Usage could not be loaded" apiError={error}>
			{#snippet action()}
				<Button variant="ghost" class="px-2 underline" onclick={() => load(search)}
					>Try again</Button
				>
			{/snippet}
		</StateMessage>
	{:else if summary && timeseries && totals}
		{#if totals.requests === 0}
			<UsageOverviewEmpty period={search.period} onperiod={(value) => change('period', value)} />
		{:else}
			<UsageTotalsTiles {totals} />

			<UsageOverviewCharts {buckets} />

			<!-- The pair sits with the series charts and above the table: it answers the same question at the
			     same altitude as the series (what consumed the window) before the table lists the rows. It
			     reads the two dimensions it draws, and reuses `groups` when the operator's own breakdown is
			     one of them, so the API's order is what breaks a tie in the ranking. -->
			<UsageOverviewBarCharts
				from={summary.from}
				to={summary.to}
				granularity={timeseries.granularity}
				groupBy={search.groupBy}
				groups={summary.groups}
				{providerNames}
			/>

			<UsageOverviewBreakdown
				breakdown={search.groupBy}
				sort={search.sort}
				order={search.order}
				{groups}
				{providerNames}
				{recordPath}
				onsort={sortBy}
			/>
		{/if}
	{/if}
</div>

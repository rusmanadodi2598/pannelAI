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
	// with one stated exception: the token series adds tokens in to tokens out, and its caption says so.
	// `latency_ms` is returned by the API and deliberately not shown, because it is a sum of per-request
	// latencies rather than a mean, and §6.5 asks for p50 and p95 rather than a total.
	//
	// The breakdown table is on screen from the first read (draft 014 F1): the API's group block arrives
	// with the same response, and "which models consumed this window" is the first question it answers.
	// Turning it off is an explicit choice the URL names.
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import UsageLivePanel from '$lib/components/UsageLivePanel.svelte';
	import UsageOverviewBreakdown from '$lib/components/UsageOverviewBreakdown.svelte';
	import UsageOverviewCharts from '$lib/components/UsageOverviewCharts.svelte';
	import UsageOverviewControls from '$lib/components/UsageOverviewControls.svelte';
	import UsageTotalsTiles from '$lib/components/UsageTotalsTiles.svelte';
	import { listProviders } from '$lib/api/providers';
	import { getUsageSummary, getUsageTimeseries } from '$lib/api/usage';
	import { type UsageSort, type UsageSummary, type UsageTimeseries } from '$lib/schemas/usage';
	import { granularityFor, periodRange, sortGroups, usageQuery } from '$lib/schemas/usage-view';
	import { providerNameMap } from '$lib/schemas/usage-topology-view';
	import { nextUsageSearch, parseUsageSearch, type UsageSearch } from '$lib/schemas/usage-search';
	import { formatTimestamp } from '$lib/utils/time';

	let summary = $state<UsageSummary | null>(null);
	let timeseries = $state<UsageTimeseries | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	// The breakdown table resolves a provider key to its name, which is a second read and not a dependency:
	// a registry that cannot be read leaves the table rendering the ids, with one line saying so. It runs
	// only while the provider dimension is the one on screen, so a model breakdown reads nothing extra.
	let providerNames = $state<Map<string, string> | null>(null);
	let namesNotice = $state<string | null>(null);

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
			error = summaryResult.error.message;
			return;
		}
		if (!seriesResult.ok) {
			error = seriesResult.error.message;
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

	function navigate(params: URLSearchParams): void {
		const next = params.toString();
		void goto(resolve(next === '' ? '/usage' : `/usage?${next}`), { keepFocus: true });
	}

	function change(key: string, value: string): void {
		navigate(nextUsageSearch(page.url.searchParams, key, value));
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

	async function loadProviderNames(): Promise<void> {
		const result = await listProviders({ per_page: 100 });

		if (!result.ok) {
			providerNames = null;
			namesNotice = `Provider names could not be read (${result.error.message}), so provider ids are shown.`;
			return;
		}

		namesNotice = null;
		providerNames = providerNameMap(result.data.data);
	}

	function refresh(): void {
		void load(search);
		if (search.groupBy === 'provider') void loadProviderNames();
	}

	// Names are read when, and only when, the provider dimension is the one on screen. The effect also
	// covers the operator switching to it, which is the moment the ids stop being readable enough.
	$effect(() => {
		if (search.groupBy !== 'provider') return;
		untrack(() => void loadProviderNames());
	});
</script>

<div class="flex flex-col gap-5">
	<UsageOverviewControls {search} onchange={change} onrefresh={refresh} />

	{#if loading}
		<StateMessage kind="loading" title="Loading usage" />
	{:else if error}
		<StateMessage kind="error" title="Usage could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => load(search)}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if summary && timeseries && totals}
		{#if totals.requests === 0}
			<StateMessage
				kind="empty"
				title="No requests in this window"
				description="Nothing was routed in the selected period. If clients are sending requests, check that a gateway key is active and that an endpoint is healthy."
			>
				{#snippet action()}
					<div class="flex flex-wrap gap-3">
						{#if search.period !== '60d'}
							<button type="button" class="underline" onclick={() => change('period', '60d')}
								>Look back 60 days</button
							>
						{/if}
						<a href={resolve('/endpoint-keys')} class="underline">Open Endpoint &amp; Key</a>
					</div>
				{/snippet}
			</StateMessage>
		{:else}
			<p class="text-sm text-[var(--color-text-muted)]">
				Showing {formatTimestamp(summary.from)} to {formatTimestamp(summary.to)}, bucketed by
				{timeseries.granularity}.
			</p>

			<UsageTotalsTiles {totals} />

			<UsageOverviewCharts {buckets} />

			<UsageOverviewBreakdown
				breakdown={search.groupBy}
				sort={search.sort}
				order={search.order}
				{groups}
				{providerNames}
				{namesNotice}
				onsort={sortBy}
			/>
		{/if}
	{/if}

	<!-- The live half of the tab, and the one part of it that is outside the branches above. It reads the
	     stream itself and renders in its own states, so a failed or empty aggregate read cannot take it
	     off the screen, which is exactly when an operator most wants to see what is happening now. -->
	<UsageLivePanel />
</div>

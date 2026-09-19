<script lang="ts">
	// The Usage Overview tab (docs/SPEC-UI/001-SPEC-UI.md §6.5).
	//
	// The period and the group-by are URL state. §8.4.2 makes a filtered view shareable, and here it also
	// keeps the window honest: the URL carries the period *name* rather than the instants it resolves to, so
	// a shared link keeps meaning "the last 7 days" instead of freezing whichever hour it was read in. Each
	// read resolves the range from one captured now, which is what makes two reads of the same period
	// comparable.
	//
	// The screen computes no figure of its own. Every number here comes from the summary or the timeseries,
	// with one stated exception: the token series adds tokens in to tokens out, and its caption says so.
	// `latency_ms` is returned by the API and deliberately not shown, because it is a sum of per-request
	// latencies rather than a mean, and §6.5 asks for p50 and p95 rather than a total.
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { untrack } from 'svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import UsageChart from '$lib/components/UsageChart.svelte';
	import UsageGroupTable from '$lib/components/UsageGroupTable.svelte';
	import UsageTotalsTiles from '$lib/components/UsageTotalsTiles.svelte';
	import { getUsageSummary, getUsageTimeseries } from '$lib/api/usage';
	import {
		USAGE_GROUP_BYS,
		USAGE_GROUP_BY_LABELS,
		USAGE_PERIODS,
		USAGE_PERIOD_LABELS,
		type UsageSummary,
		type UsageTimeseries
	} from '$lib/schemas/usage';
	import {
		granularityFor,
		periodRange,
		usageQuery,
		type SeriesPoint
	} from '$lib/schemas/usage-view';
	import { nextUsageSearch, parseUsageSearch, type UsageSearch } from '$lib/schemas/usage-search';
	import { formatTimestamp } from '$lib/utils/time';

	let summary = $state<UsageSummary | null>(null);
	let timeseries = $state<UsageTimeseries | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

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

	const requests = $derived<SeriesPoint[]>(
		buckets.map((bucket) => ({ bucket: bucket.bucket, value: bucket.totals.requests }))
	);
	const tokens = $derived<SeriesPoint[]>(
		buckets.map((bucket) => ({
			bucket: bucket.bucket,
			value: bucket.totals.tokens_in + bucket.totals.tokens_out
		}))
	);

	function change(key: string, value: string): void {
		const next = nextUsageSearch(page.url.searchParams, key, value).toString();
		void goto(resolve(next === '' ? '/usage' : `/usage?${next}`), { keepFocus: true });
	}
</script>

<div class="flex flex-col gap-5">
	<div class="flex flex-wrap items-end gap-3">
		<label class="flex flex-col gap-1 text-sm">
			<span class="text-[var(--color-text-muted)]">Period</span>
			<select
				value={search.period}
				onchange={(event) => change('period', event.currentTarget.value)}
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
			>
				{#each USAGE_PERIODS as period (period)}
					<option value={period}>{USAGE_PERIOD_LABELS[period]}</option>
				{/each}
			</select>
		</label>

		<label class="flex flex-col gap-1 text-sm">
			<span class="text-[var(--color-text-muted)]">Group by</span>
			<select
				value={search.groupBy}
				onchange={(event) => change('group_by', event.currentTarget.value)}
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
			>
				<option value="">No breakdown</option>
				{#each USAGE_GROUP_BYS as option (option)}
					<option value={option}>{USAGE_GROUP_BY_LABELS[option]}</option>
				{/each}
			</select>
		</label>
	</div>

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

			<div class="grid gap-4 lg:grid-cols-2">
				<UsageChart
					title="Requests"
					unit="requests"
					caption="Requests recorded in each bucket."
					points={requests}
				/>
				<UsageChart
					title="Tokens"
					unit="tokens"
					caption="Tokens in plus tokens out in each bucket. Cache tokens are counted in the tiles above and not here."
					points={tokens}
				/>
			</div>

			{#if search.groupBy !== ''}
				{#if summary.groups.length === 0}
					<StateMessage
						kind="empty"
						title="No group has any usage in this window"
						description="The totals above are real, so the breakdown is empty because nothing was recorded against a {USAGE_GROUP_BY_LABELS[
							search.groupBy
						].toLowerCase()}. Check that the requests carried one."
					/>
				{:else}
					<UsageGroupTable groups={summary.groups} groupBy={search.groupBy} />
				{/if}
			{/if}
		{/if}
	{/if}
</div>

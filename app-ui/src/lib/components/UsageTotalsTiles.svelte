<script lang="ts">
	// The usage totals as two tiers (docs/SPEC-UI/001-SPEC-UI.md §6.5).
	//
	// The nine figures §6.5 lists are all here, in that order, and every one comes from the summary response.
	// The hierarchy is the point: Requests, Cost, and the token total are the figures an operator decides
	// from, so they lead at a larger weight. The six subdivisions sit in one dense strip below, on a single
	// surface, where they are read after the decision rather than beside it.
	//
	// Two figures carry a note rather than a bare number: the cost, because SPEC-API §7.12 states cost
	// figures are estimates for display, and Requests, because a failure count is easier to trust beside
	// the total it belongs to.
	//
	// `latency_ms` is not among them. The API returns it as a sum of per-request latencies rather than a
	// mean, and §6.5 asks for p50 and p95, so a tile labelled "latency" would state something the field does
	// not mean. The two percentiles share one Reliability cell for the same reason: they are one answer.
	import type { UsageTotals } from '$lib/schemas/usage';
	import { errorRatePercent, formatCount } from '$lib/schemas/usage-view';

	let { totals }: { totals: UsageTotals } = $props();

	const lead = $derived([
		{
			label: 'Requests',
			value: formatCount(totals.requests),
			note: `${formatCount(totals.error_count)} of ${formatCount(totals.requests)} failed.`,
			span: ''
		},
		{
			label: 'Cost (USD)',
			value: totals.cost_usd,
			note: 'An estimate for display, not a billing figure.',
			span: ''
		},
		{
			label: 'Tokens',
			value: formatCount(totals.tokens_in + totals.tokens_out),
			note: 'Tokens in plus tokens out.',
			span: 'col-span-2 sm:col-span-1'
		}
	]);

	const support = $derived([
		{ label: 'Tokens in', value: formatCount(totals.tokens_in), note: '' },
		{ label: 'Tokens out', value: formatCount(totals.tokens_out), note: '' },
		{ label: 'Cache read', value: formatCount(totals.tokens_cache_read), note: '' },
		{ label: 'Cache write', value: formatCount(totals.tokens_cache_write), note: '' },
		{ label: 'Error rate', value: errorRatePercent(totals.error_rate), note: '' },
		{
			label: 'Reliability',
			value: `${formatCount(totals.latency_p50_ms)} / ${formatCount(totals.latency_p95_ms)} ms`,
			note: 'Latency p50 / p95.'
		}
	]);
</script>

<div class="flex flex-col gap-2">
	<dl class="grid grid-cols-2 gap-3 sm:grid-cols-3">
		{#each lead as tile (tile.label)}
			<div
				class={`flex min-w-0 flex-col gap-1 rounded-[var(--radius-md)] border border-[var(--color-border)] px-3 py-2 ${tile.span}`}
			>
				<dt class="text-sm text-[var(--color-text-muted)]">{tile.label}</dt>
				<dd class="break-words text-xl font-semibold tabular-nums">{tile.value}</dd>
				{#if tile.note}
					<p class="text-xs text-[var(--color-text-muted)]">{tile.note}</p>
				{/if}
			</div>
		{/each}
	</dl>

	<dl
		class="grid grid-cols-2 gap-x-4 gap-y-2 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] px-3 py-2 sm:grid-cols-3"
	>
		{#each support as tile (tile.label)}
			<div class="flex min-w-0 flex-col gap-0.5">
				<dt class="text-xs text-[var(--color-text-muted)]">{tile.label}</dt>
				<dd class="break-words text-sm font-medium tabular-nums">{tile.value}</dd>
				{#if tile.note}
					<p class="text-xs text-[var(--color-text-muted)]">{tile.note}</p>
				{/if}
			</div>
		{/each}
	</dl>
</div>

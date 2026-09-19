<script lang="ts">
	// The usage totals as tiles (docs/SPEC-UI/001-SPEC-UI.md §6.5).
	//
	// The tiles are exactly the figures §6.5 lists, in that order, and every one of them comes from the
	// summary response. Two carry a note rather than a bare number: the cost, because SPEC-API §7.12 states
	// cost figures are estimates for display, and the error rate, because a percentage is easier to trust
	// when the two counters behind it are visible.
	//
	// `latency_ms` is not among them. The API returns it as a sum of per-request latencies rather than a
	// mean, and §6.5 asks for p50 and p95, so a tile labelled "latency" would state something the field does
	// not mean.
	import type { UsageTotals } from '$lib/schemas/usage';
	import { errorRatePercent, formatCount } from '$lib/schemas/usage-view';

	let { totals }: { totals: UsageTotals } = $props();

	const tiles = $derived([
		{ label: 'Requests', value: formatCount(totals.requests), note: '' },
		{ label: 'Tokens in', value: formatCount(totals.tokens_in), note: '' },
		{ label: 'Tokens out', value: formatCount(totals.tokens_out), note: '' },
		{ label: 'Cache read', value: formatCount(totals.tokens_cache_read), note: '' },
		{ label: 'Cache write', value: formatCount(totals.tokens_cache_write), note: '' },
		{
			label: 'Cost (USD)',
			value: totals.cost_usd,
			note: 'An estimate for display, not a billing figure.'
		},
		{
			label: 'Error rate',
			value: errorRatePercent(totals.error_rate),
			note: `${formatCount(totals.error_count)} of ${formatCount(totals.requests)} failed.`
		},
		{ label: 'Latency p50', value: `${formatCount(totals.latency_p50_ms)} ms`, note: '' },
		{ label: 'Latency p95', value: `${formatCount(totals.latency_p95_ms)} ms`, note: '' }
	]);
</script>

<dl class="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
	{#each tiles as tile (tile.label)}
		<div
			class="flex flex-col gap-1 rounded-[var(--radius-md)] border border-[var(--color-border)] px-3 py-2"
		>
			<dt class="text-sm text-[var(--color-text-muted)]">{tile.label}</dt>
			<dd class="text-lg font-medium tabular-nums">{tile.value}</dd>
			{#if tile.note}
				<p class="text-xs text-[var(--color-text-muted)]">{tile.note}</p>
			{/if}
		</div>
	{/each}
</dl>

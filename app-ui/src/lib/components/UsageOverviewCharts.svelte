<script lang="ts">
	// The Overview tab's charts (docs/SPEC-UI/001-SPEC-UI.md §6.5, draft 014 F2).
	//
	// Two cards: the requests series, and one value series that switches between tokens and cost. The
	// reference's chart carries the same Tokens/Cost switch, and it is local state rather than a URL
	// parameter because it changes no read: the same two responses feed both modes.
	//
	// The cost series parses the API's decimal string, which is the only place on this screen that does.
	// The tiles print that string as it arrives, and a chart needs a number to size a bar.
	import UsageChart from '$lib/components/UsageChart.svelte';
	import type { UsageBucket } from '$lib/schemas/usage';
	import { formatCost, type SeriesPoint } from '$lib/schemas/usage-view';

	let { buckets }: { buckets: UsageBucket[] } = $props();

	const requests = $derived<SeriesPoint[]>(
		buckets.map((bucket) => ({ bucket: bucket.bucket, value: bucket.totals.requests }))
	);
	const tokens = $derived<SeriesPoint[]>(
		buckets.map((bucket) => ({
			bucket: bucket.bucket,
			value: bucket.totals.tokens_in + bucket.totals.tokens_out
		}))
	);
	const costs = $derived<SeriesPoint[]>(
		buckets.map((bucket) => ({
			bucket: bucket.bucket,
			value: Number.parseFloat(bucket.totals.cost_usd)
		}))
	);

	let chartMode = $state<'tokens' | 'costs'>('tokens');

	const valueChart = $derived(
		chartMode === 'costs'
			? {
					title: 'Cost (USD)',
					unit: 'USD',
					caption:
						'Estimated cost in each bucket. These figures are estimates for display, not billing amounts.',
					points: costs,
					format: formatCost
				}
			: {
					title: 'Tokens',
					unit: 'tokens',
					caption:
						'Tokens in plus tokens out in each bucket. Cache tokens are counted in the tiles above and not here.',
					points: tokens
				}
	);

	const toggleClass =
		'min-h-11 rounded-[var(--radius-sm)] px-3 text-sm aria-pressed:bg-[var(--color-surface-2)]';
</script>

<div class="grid gap-4 lg:grid-cols-2">
	<UsageChart
		title="Requests"
		unit="requests"
		caption="Requests recorded in each bucket."
		points={requests}
	/>
	<UsageChart {...valueChart}>
		{#snippet controls()}
			<div
				class="flex items-center gap-1 rounded-[var(--radius-sm)] border border-[var(--color-border)] p-1"
			>
				<button
					type="button"
					aria-pressed={chartMode === 'tokens'}
					class={toggleClass}
					onclick={() => (chartMode = 'tokens')}>Tokens</button
				>
				<button
					type="button"
					aria-pressed={chartMode === 'costs'}
					class={toggleClass}
					onclick={() => (chartMode = 'costs')}>Cost</button
				>
			</div>
		{/snippet}
	</UsageChart>
</div>

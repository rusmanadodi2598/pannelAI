<script lang="ts">
	// The Overview tab's charts (docs/SPEC-UI/001-SPEC-UI.md §6.5).
	//
	// Two cards: the requests series, and one value series that switches between tokens and cost. The
	// reference's chart carries the same Tokens/Cost switch, and it is local state rather than a URL
	// parameter because it changes no read: the same two responses feed both modes.
	//
	// The cost series parses the API's decimal string, which is the only place on this screen that does.
	// The tiles print that string as it arrives, and a chart needs a number to size a bar.
	import UsageChart from '$lib/components/UsageChart.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import type { UsageBucket } from '$lib/schemas/usage';
	import { formatCost } from '$lib/schemas/usage-view';
	import { type SeriesPoint } from '$lib/schemas/usage-series';

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
					caption: 'Estimated cost per bucket, not a billing figure.',
					points: costs,
					format: formatCost
				}
			: {
					title: 'Tokens',
					unit: 'tokens',
					caption: 'Tokens in plus tokens out per bucket. Cache is in the tiles.',
					points: tokens
				}
	);
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
				<Button
					aria-pressed={chartMode === 'tokens'}
					variant="toggle"
					class="border-transparent"
					onclick={() => (chartMode = 'tokens')}>Tokens</Button
				>
				<Button
					aria-pressed={chartMode === 'costs'}
					variant="toggle"
					class="border-transparent"
					onclick={() => (chartMode = 'costs')}>Cost</Button
				>
			</div>
		{/snippet}
	</UsageChart>
</div>

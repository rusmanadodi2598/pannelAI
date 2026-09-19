<script lang="ts">
	// The group-by breakdown of a usage summary (docs/SPEC-UI/001-SPEC-UI.md §6.5).
	//
	// The first column is named by the dimension the operator chose, because the API returns a bare `key`
	// and a column headed "Key" would leave the reader to remember which selector is set.
	//
	// The group key is rendered as it arrives. For provider, endpoint, and gateway key it is an identifier
	// rather than a label, and the panel does not resolve it here: the ids are what the API groups by, and
	// the endpoint and provider screens are where a name is looked up.
	import { USAGE_GROUP_BY_LABELS, type UsageGroup, type UsageGroupBy } from '$lib/schemas/usage';
	import { errorRatePercent, formatCount } from '$lib/schemas/usage-view';

	let { groups, groupBy }: { groups: UsageGroup[]; groupBy: UsageGroupBy } = $props();

	const dimension = $derived(USAGE_GROUP_BY_LABELS[groupBy]);
</script>

<div class="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]">
	<table class="w-full min-w-[44rem] border-collapse text-sm">
		<caption class="sr-only">Usage broken down by {dimension.toLowerCase()}</caption>
		<thead class="bg-[var(--color-surface-2)] text-left">
			<tr>
				<th scope="col" class="px-3 py-2 font-medium">{dimension}</th>
				<th scope="col" class="px-3 py-2 font-medium">Requests</th>
				<th scope="col" class="px-3 py-2 font-medium">Tokens in</th>
				<th scope="col" class="px-3 py-2 font-medium">Tokens out</th>
				<th scope="col" class="px-3 py-2 font-medium">Cost (USD)</th>
				<th scope="col" class="px-3 py-2 font-medium">Errors</th>
				<th scope="col" class="px-3 py-2 font-medium">Error rate</th>
			</tr>
		</thead>
		<tbody>
			{#each groups as group (group.key)}
				<tr class="border-t border-[var(--color-border)]">
					<td class="px-3 py-2">{group.key}</td>
					<td class="px-3 py-2 tabular-nums">{formatCount(group.totals.requests)}</td>
					<td class="px-3 py-2 tabular-nums">{formatCount(group.totals.tokens_in)}</td>
					<td class="px-3 py-2 tabular-nums">{formatCount(group.totals.tokens_out)}</td>
					<td class="px-3 py-2 tabular-nums">{group.totals.cost_usd}</td>
					<td class="px-3 py-2 tabular-nums">{formatCount(group.totals.error_count)}</td>
					<td class="px-3 py-2 tabular-nums">{errorRatePercent(group.totals.error_rate)}</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

<script lang="ts">
	// The group-by breakdown of a usage summary (docs/SPEC-UI/001-SPEC-UI.md §6.5, draft 014 F1).
	//
	// The first column is named by the dimension the operator chose, because the API returns a bare `key`
	// and a column headed "Key" would leave the reader to remember which selector is set.
	//
	// A provider key is resolved to its name when the registry was read, because `openai` is a name the
	// operator has to translate and the reference resolves it too. Every other dimension is rendered as it
	// arrives: the endpoint and gateway-key screens are where those names are looked up, and this tab reads
	// neither, which is a deviation from the reference recorded in draft 014 F6.
	//
	// The table carries no Costs/Tokens toggle of its own. The reference's exists because its value columns
	// switch between a token view and a cost view, and switching here would hide figures this table already
	// shows at once; the toggle belongs to the chart, which can draw one series at a time (draft 014 F6, F2).
	//
	// The sort is view state and lives in the URL (§8.4.2), so a sorted breakdown is a view someone can
	// share. It orders the whole response rather than a page of it, because the API sends the complete
	// breakdown for the window.
	import {
		USAGE_GROUP_BY_LABELS,
		type UsageGroup,
		type UsageGroupBy,
		type UsageOrder,
		type UsageSort
	} from '$lib/schemas/usage';
	import { errorRatePercent, formatCount } from '$lib/schemas/usage-view';

	type Props = {
		groups: UsageGroup[];
		groupBy: UsageGroupBy;
		sort: UsageSort | '';
		order: UsageOrder;
		/** The registry's id-to-name map, or null when it could not be read and the ids are shown instead. */
		providerNames: Map<string, string> | null;
		/** The column a header click asks for, or an empty string when the sort is being cleared. */
		onsort: (field: UsageSort | '') => void;
	};

	let { groups, groupBy, sort, order, providerNames, onsort }: Props = $props();

	const dimension = $derived(USAGE_GROUP_BY_LABELS[groupBy]);

	function keyOf(group: UsageGroup): string {
		if (groupBy !== 'provider' || providerNames === null) return group.key;
		return providerNames.get(group.key.toLowerCase()) ?? group.key;
	}

	function ariaSort(field: UsageSort): 'ascending' | 'descending' | 'none' {
		if (sort !== field) return 'none';
		return order === 'asc' ? 'ascending' : 'descending';
	}

	function mark(field: UsageSort): string {
		if (sort !== field) return '';
		return order === 'asc' ? ' ↑' : ' ↓';
	}
</script>

{#snippet head(field: UsageSort, label: string)}
	<th scope="col" class="px-3 py-2 text-right font-medium" aria-sort={ariaSort(field)}>
		<button type="button" class="underline decoration-dotted" onclick={() => onsort(field)}>
			{label}{mark(field)}
		</button>
	</th>
{/snippet}

{#if sort !== ''}
	<div class="flex justify-end">
		<button type="button" class="min-h-11 underline" onclick={() => onsort('')}>Clear sort</button>
	</div>
{/if}

<div class="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]">
	<table class="w-full min-w-[44rem] border-collapse text-sm">
		<caption class="sr-only">Usage broken down by {dimension.toLowerCase()}</caption>
		<thead class="bg-[var(--color-surface-2)] text-left">
			<tr>
				<th scope="col" class="px-3 py-2 font-medium" aria-sort={ariaSort('key')}>
					<button type="button" class="underline decoration-dotted" onclick={() => onsort('key')}
						>{dimension}{mark('key')}</button
					>
				</th>
				{@render head('requests', 'Requests')}
				{@render head('tokens_in', 'Tokens in')}
				{@render head('tokens_out', 'Tokens out')}
				{@render head('cost_usd', 'Cost (USD)')}
				{@render head('error_count', 'Errors')}
				{@render head('error_rate', 'Error rate')}
			</tr>
		</thead>
		<tbody>
			{#each groups as group (group.key)}
				<tr class="border-t border-[var(--color-border)]">
					<td class="px-3 py-2">{keyOf(group)}</td>
					<td class="px-3 py-2 text-right tabular-nums">{formatCount(group.totals.requests)}</td>
					<td class="px-3 py-2 text-right tabular-nums">{formatCount(group.totals.tokens_in)}</td>
					<td class="px-3 py-2 text-right tabular-nums">{formatCount(group.totals.tokens_out)}</td>
					<td class="px-3 py-2 text-right tabular-nums">{group.totals.cost_usd}</td>
					<td class="px-3 py-2 text-right tabular-nums">
						{formatCount(group.totals.error_count)}
					</td>
					<td class="px-3 py-2 text-right tabular-nums">
						{errorRatePercent(group.totals.error_rate)}
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

<script lang="ts">
	// The Overview tab's breakdown of the window (docs/SPEC-UI/001-SPEC-UI.md §6.5, draft 014 F1).
	//
	// The table is on screen from the first read, because the API's group block arrives with the summary the
	// tab already asked for and "which models consumed this window" is the first question it answers.
	// Turning it off is an explicit choice the URL names, and this component renders nothing for it.
	//
	// A provider key is resolved to its registry name, which is a read of its own and the one thing here that
	// can fail alone: a registry that cannot be read leaves the ids on screen with one line saying so. The
	// read belongs to the tab, because the tab is what a refresh repeats.
	import StateMessage from '$lib/components/StateMessage.svelte';
	import UsageGroupTable from '$lib/components/UsageGroupTable.svelte';
	import {
		USAGE_BREAKDOWN_NONE,
		USAGE_GROUP_BY_LABELS,
		type UsageBreakdown,
		type UsageGroup,
		type UsageOrder,
		type UsageSort
	} from '$lib/schemas/usage';

	type Props = {
		breakdown: UsageBreakdown;
		sort: UsageSort | '';
		order: UsageOrder;
		/** The groups the summary returned, already in the order the URL asked for. */
		groups: UsageGroup[];
		/** The registry's id-to-name map, or null when it could not be read. */
		providerNames: Map<string, string> | null;
		namesNotice: string | null;
		onsort: (field: UsageSort | '') => void;
	};

	let { breakdown, sort, order, groups, providerNames, namesNotice, onsort }: Props = $props();
</script>

{#if breakdown !== USAGE_BREAKDOWN_NONE}
	{#if breakdown === 'provider' && namesNotice !== null}
		<p role="status" class="text-sm text-[var(--color-text-muted)]">{namesNotice}</p>
	{/if}

	{#if groups.length === 0}
		<StateMessage
			kind="empty"
			title="No group has any usage in this window"
			description="The totals above are real, so the breakdown is empty because nothing was recorded against a {USAGE_GROUP_BY_LABELS[
				breakdown
			].toLowerCase()}. Check that the requests carried one."
		/>
	{:else}
		<UsageGroupTable {groups} groupBy={breakdown} {sort} {order} {providerNames} {onsort} />
	{/if}
{/if}

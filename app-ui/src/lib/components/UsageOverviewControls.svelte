<script lang="ts">
	// The Overview tab's controls (docs/SPEC-UI/001-SPEC-UI.md §6.5).
	//
	// Both selects commit on change, because a select has no half-typed state to protect, and the URL
	// corrections the parser made are printed beside them rather than swallowed (§8.10, R-27).
	//
	// The group-by selector carries the four dimensions and then "No breakdown", which is the screen's own
	// word for the API's absence of a `group_by`. It defaults to Model, so the breakdown
	// table is on screen from the first read.
	import RefreshControl from '$lib/components/RefreshControl.svelte';
	import {
		USAGE_BREAKDOWN_NONE,
		USAGE_GROUP_BYS,
		USAGE_GROUP_BY_LABELS,
		USAGE_PERIODS,
		USAGE_PERIOD_LABELS
	} from '$lib/schemas/usage';
	import type { UsageSearch } from '$lib/schemas/usage-search';

	type Props = {
		search: UsageSearch;
		onchange: (key: string, value: string) => void;
		onrefresh: () => void;
	};

	let { search, onchange, onrefresh }: Props = $props();

	const fieldClass =
		'min-h-11 w-full min-w-0 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm';
</script>

<div class="grid grid-cols-3 items-end gap-2">
	<label class="flex min-w-0 flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]">Period</span>
		<select
			value={search.period}
			onchange={(event) => onchange('period', event.currentTarget.value)}
			class={fieldClass}
		>
			{#each USAGE_PERIODS as period (period)}
				<option value={period}>{USAGE_PERIOD_LABELS[period]}</option>
			{/each}
		</select>
	</label>

	<label class="flex min-w-0 flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]">Group by</span>
		<select
			value={search.groupBy}
			onchange={(event) => onchange('group_by', event.currentTarget.value)}
			class={fieldClass}
		>
			{#each USAGE_GROUP_BYS as option (option)}
				<option value={option}>{USAGE_GROUP_BY_LABELS[option]}</option>
			{/each}
			<option value={USAGE_BREAKDOWN_NONE}>No breakdown</option>
		</select>
	</label>

	<!-- The refresh takes the third column, so the band reads as three equal controls. Its label row matches
	     the two selects' label row, which keeps the three controls on one line. The rule lives here rather
	     than in RefreshControl, which other screens render at their own size: on a phone the third column is
	     too narrow for the icon and the full word together, so the glyph steps aside and the label stays on
	     one line. From `sm` the column is wide enough for both. -->
	<div class="flex min-w-0 flex-col gap-1 text-sm">
		<span class="text-[var(--color-text-muted)]" aria-hidden="true">Refresh</span>
		<div
			class="flex [&>div]:w-full [&_button]:w-full [&_button]:justify-center [&_button]:px-2 [&_button]:whitespace-nowrap [&_svg]:hidden sm:[&_svg]:block"
		>
			<RefreshControl {onrefresh} />
		</div>
	</div>
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

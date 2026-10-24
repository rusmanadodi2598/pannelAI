<script lang="ts">
	// The Overview tab's controls (docs/SPEC-UI/001-SPEC-UI.md §6.5).
	//
	// Both selects commit on change, because a select has no half-typed state to protect, and the URL
	// corrections the parser made are printed beside them rather than swallowed (§8.10, R-27).
	//
	// The group-by selector carries the four dimensions and then "No breakdown", which is the screen's own
	// word for the API's absence of a `group_by`. It defaults to Model, so the breakdown
	// table is on screen from the first read.
	//
	// The query inputs and the window they produced are one statement, so they share one surface: the
	// resolved range and the granularity sit at the trailing edge in tabular figures.
	import RefreshControl from '$lib/components/RefreshControl.svelte';
	import {
		USAGE_BREAKDOWN_NONE,
		USAGE_GROUP_BYS,
		USAGE_GROUP_BY_LABELS,
		USAGE_PERIODS,
		USAGE_PERIOD_LABELS
	} from '$lib/schemas/usage';
	import type { UsageSearch } from '$lib/schemas/usage-search';
	import { formatTimestampWithTime } from '$lib/utils/time';

	type Props = {
		search: UsageSearch;
		/** The resolved window, or null while there is no summary to name it. */
		window: { from: string; to: string; granularity: string } | null;
		onchange: (key: string, value: string) => void;
		onrefresh: () => void;
	};

	let { search, window, onchange, onrefresh }: Props = $props();

	const fieldClass =
		'min-h-11 w-full min-w-0 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm';
</script>

<div
	class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] p-3 sm:flex-row sm:items-end"
>
	<div class="grid grid-cols-1 gap-2 sm:grid-cols-[minmax(0,10rem)_minmax(0,12rem)_auto]">
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

		<!-- The refresh takes the trailing control column. Its label row matches the two selects' label row,
		     which keeps the controls on one line from sm. -->
		<div class="flex min-w-0 flex-col gap-1 text-sm">
			<span class="text-[var(--color-text-muted)]" aria-hidden="true">Refresh</span>
			<div
				class="flex [&>div]:w-full [&_button]:w-full [&_button]:justify-center [&_button]:whitespace-nowrap sm:[&_button]:justify-start"
			>
				<RefreshControl {onrefresh} />
			</div>
		</div>
	</div>

	{#if window}
		<p
			class="text-xs text-[var(--color-text-muted)] tabular-nums sm:ms-auto sm:max-w-lg sm:text-right"
		>
			Showing {formatTimestampWithTime(window.from)} to {formatTimestampWithTime(window.to)},
			bucketed by {window.granularity}.
		</p>
	{/if}
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

<script lang="ts">
	// The quota screen's toolbar (docs/SPEC-UI/001-SPEC-UI.md §6.6, §8.6.1; PORT 006's toolbar controls).
	//
	// One compact row for what the operator reaches for together: the status
	// sentence, the seconds to the next read, the provider filter, and the two refresh controls share one
	// baseline, and the row wraps as a block on a narrow screen. The controls carry the icon map's glyphs
	// beside their labels (R-04, R-31).
	//
	// The countdown is the visible half of §8.6.1's "the interval must be visible": an operator who can see
	// the next read coming does not press Refresh to find out whether the screen is alive. It is fed the
	// screen's own clock and pause state, so it cannot claim a read the pause has forbidden.
	//
	// The filter narrows the cards on this page and sends nothing: the read stays server-paged and
	// unfiltered, because a filter that re-paged the wire would make the pager answer to the filter.
	//
	// The refresh control is this screen's own rather than the shared `RefreshControl`, because the two
	// screens have different rules about a second press: the shared one drops it, and this one queues it so a
	// press during a poll still reads once the poll lands.
	import { CONTROL_ICONS } from '$lib/icons';
	import { Button } from '$lib/primitives/button/index.js';
	import { pollIntervalLabel, QUOTA_POLL_MS } from '$lib/polling';
	import { formatTimestampWithTime } from '$lib/utils/time';

	type Props = {
		paused: boolean;
		/** When the last read landed, empty until one has. */
		readAt: string;
		/** Seconds to the next automatic read, or null when there is no read coming to count down to. */
		secondsToNext: number | null;
		/** The providers this page's cards name, in first-seen order. */
		providers: string[];
		/** The provider the cards are narrowed to, empty for all of them. */
		provider: string;
		/** Whether a read is in flight, which the button reports in its own label. */
		refreshing?: boolean;
		onproviderchange: (next: string) => void;
		onrefresh: () => void;
		ontogglepause: () => void;
	};

	let {
		paused,
		readAt,
		secondsToNext,
		providers,
		provider,
		refreshing = false,
		onproviderchange,
		onrefresh,
		ontogglepause
	}: Props = $props();

	const RefreshIcon = CONTROL_ICONS.refresh.icon;
	const PauseIcon = CONTROL_ICONS.pause.icon;
	const ResumeIcon = CONTROL_ICONS.resume.icon;
</script>

<!-- Same rule as the combo toolbar (antislop-layoutmobile, Action Pair and Tap Targets): the sentence takes
     its own line on mobile, and the controls align center, so a tall sentence cannot stretch the buttons. -->
<div class="flex flex-wrap items-center justify-between gap-2">
	<p
		class="flex min-w-0 basis-full items-center text-sm text-[var(--color-text-muted)] sm:flex-1 sm:basis-auto"
	>
		{#if paused}
			Refresh is paused. Countdowns still tick.
		{:else}
			Auto-refresh {pollIntervalLabel(QUOTA_POLL_MS)}.
		{/if}
	</p>

	{#if readAt}
		<p class="flex items-center text-sm text-[var(--color-text-muted)]">
			Last read {formatTimestampWithTime(readAt)}.
		</p>
	{/if}

	<div class="flex flex-wrap items-center gap-2">
		<label class="flex items-center gap-2 text-sm">
			<span class="text-[var(--color-text-muted)]">Provider</span>
			<select
				aria-label="Filter cards by provider"
				title={provider || 'All providers'}
				class="min-h-11 w-40 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
				value={provider}
				onchange={(event) => onproviderchange(event.currentTarget.value)}
			>
				<option value="">All providers</option>
				{#each providers as name (name)}
					<option value={name}>{name}</option>
				{/each}
			</select>
		</label>

		{#if secondsToNext !== null}
			<p
				role="status"
				class="flex items-center text-sm tabular-nums text-[var(--color-text-muted)]"
			>
				Next read in {secondsToNext}s
			</p>
		{/if}

		<!-- Not disabled while a read is in flight: the screen allows one read at a time, so the control
		     always asks and the guard decides. A disabled gate here would swallow the click when a poll is
		     mid-flight, which reads as a broken button. -->
		<Button variant="outline" aria-busy={refreshing} onclick={onrefresh}>
			<RefreshIcon class="size-4" aria-hidden="true" />
			{refreshing ? 'Refreshing' : 'Refresh now'}
		</Button>

		<Button variant="toggle" aria-pressed={paused} onclick={ontogglepause}>
			{#if paused}
				<ResumeIcon class="size-4" aria-hidden="true" />
			{:else}
				<PauseIcon class="size-4" aria-hidden="true" />
			{/if}
			{paused ? 'Resume refresh' : 'Pause refresh'}
		</Button>
	</div>
</div>

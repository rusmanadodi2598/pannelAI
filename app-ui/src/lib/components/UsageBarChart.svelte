<script lang="ts">
	// One bar chart of the Overview (docs/SPEC-UI/001-SPEC-UI.md §6.5, draft 016 F1 and F2).
	//
	// Bars are laid out in CSS rather than SVG, for the reason the series chart gives: there is no axis to
	// scale and no series to overlay, so a viewBox would add coordinate arithmetic without adding
	// information, and a flex row inherits the theme's colours and the panel's responsive behaviour.
	//
	// Both charts are horizontal, unlike the reference's provider chart, which draws vertical bars and cuts
	// the provider name at ten characters (`ProviderBarChart.js:75`). A provider or model name is long, this
	// card is one of two in a row, and a truncated name is a defect the panel does not need to copy: here
	// the name column truncates with CSS, so the full name stays in the markup and in the title attribute.
	//
	// The bars are hidden from assistive technology on purpose, because the summary sentence and the table
	// behind the disclosure state the same numbers, and a chart read twice is noise.
	//
	// The Tokens/Requests switch is local state in the caller rather than a URL parameter, because it
	// changes no read: one response feeds both modes. That is the same reason the series charts give for
	// their Tokens/Cost switch.
	import {
		USAGE_MEASURES,
		USAGE_MEASURE_LABELS,
		barsSummary,
		formatCompact,
		type UsageBarSet,
		type UsageMeasure
	} from '$lib/schemas/usage-bars';
	import { formatCount } from '$lib/schemas/usage-view';

	type Props = {
		title: string;
		/** One line naming what is drawn, so the chart is not the only statement of its own meaning. */
		caption: string;
		/** What the bars rank, in the singular, for the summary sentence and the name column. */
		noun: string;
		set: UsageBarSet;
		measure: UsageMeasure;
		onmeasure: (measure: UsageMeasure) => void;
		/** The read's own refusal. It replaces the bars rather than emptying them, which would be a claim. */
		error?: string | null;
	};

	let { title, caption, noun, set, measure, onmeasure, error = null }: Props = $props();

	const summary = $derived(barsSummary(set, { measure, noun }));
	const nameHeader = $derived(noun.charAt(0).toUpperCase() + noun.slice(1));
	// A window with requests can still have no tokens in it, so the empty line names the measure rather than
	// saying the window is empty, which the tab's own empty state already says when it is true.
	const emptyLine = $derived(`No ${noun} has ${measure} in this window.`);
	const toggleClass =
		'min-h-11 rounded-[var(--radius-sm)] px-3 text-sm aria-pressed:bg-[var(--color-surface-2)]';
</script>

<figure
	class="flex min-w-0 flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4"
>
	<figcaption class="flex flex-col gap-1">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<span class="text-sm font-medium">{title}</span>
			<div
				class="flex items-center gap-1 rounded-[var(--radius-sm)] border border-[var(--color-border)] p-1"
			>
				{#each USAGE_MEASURES as option (option)}
					<button
						type="button"
						aria-pressed={measure === option}
						class={toggleClass}
						onclick={() => onmeasure(option)}>{USAGE_MEASURE_LABELS[option]}</button
					>
				{/each}
			</div>
		</div>
		<span class="text-sm text-[var(--color-text-muted)]">{caption}</span>
	</figcaption>

	{#if error}
		<p class="text-sm text-[var(--color-danger)]">{error}</p>
	{:else if set.bars.length === 0}
		<p class="text-sm text-[var(--color-text-muted)]">{emptyLine}</p>
	{:else}
		<div class="flex flex-col gap-2" aria-hidden="true">
			{#each set.bars as bar (bar.key)}
				<div class="flex items-center gap-2">
					<span class="w-28 shrink-0 truncate text-xs" title={bar.label}>{bar.label}</span>
					<span class="min-w-0 flex-1"
						><span
							class="block h-3 rounded-r-[2px] bg-[var(--color-accent)]"
							style={`width: ${bar.width}%`}
						></span></span
					>
					<span class="w-14 shrink-0 text-right text-xs tabular-nums"
						>{formatCompact(bar.value)}</span
					>
				</div>
			{/each}
		</div>

		<p class="text-sm">{summary}</p>

		<details class="text-sm">
			<summary class="min-h-11 cursor-pointer">Show these numbers as a table</summary>
			<div class="overflow-x-auto pt-2">
				<table class="w-full border-collapse">
					<caption class="sr-only">{title} by {noun}</caption>
					<thead class="bg-[var(--color-surface-2)] text-left">
						<tr>
							<th scope="col" class="px-3 py-2 font-medium">{nameHeader}</th>
							<th scope="col" class="px-3 py-2 font-medium">Tokens</th>
							<th scope="col" class="px-3 py-2 font-medium">Requests</th>
						</tr>
					</thead>
					<tbody>
						{#each set.bars as bar (bar.key)}
							<tr class="border-t border-[var(--color-border)]">
								<td class="px-3 py-2">{bar.label}</td>
								<td class="px-3 py-2 tabular-nums">{formatCount(bar.tokens)}</td>
								<td class="px-3 py-2 tabular-nums">{formatCount(bar.requests)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</details>
	{/if}
</figure>

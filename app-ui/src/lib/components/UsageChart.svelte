<script lang="ts">
	// One usage series over time (docs/SPEC-UI/001-SPEC-UI.md §6.5).
	//
	// §6.5 requires the chart to be readable without the graphic, so the numbers live in three places and
	// the drawing is the least of them: a text summary line, a table behind a disclosure, and bars. The bars
	// are hidden from assistive technology on purpose, because a chart labelled with the same sentence the
	// summary already prints makes a screen reader read it twice.
	//
	// Bars are laid out in CSS rather than SVG. There is no axis to scale and no series to overlay, so a
	// viewBox would add coordinate arithmetic without adding information, and a flex row inherits the
	// theme's colours and the panel's own responsive behaviour for free.
	import {
		barHeights,
		formatCount,
		seriesSummary,
		type SeriesPoint
	} from '$lib/schemas/usage-view';
	import { formatTimestamp } from '$lib/utils/time';

	type Props = {
		title: string;
		/** What the series counts, in the plural, for the summary sentence. */
		unit: string;
		/** One line naming what is drawn, so the chart is not the only statement of its own meaning. */
		caption: string;
		points: SeriesPoint[];
	};

	let { title, unit, caption, points }: Props = $props();

	const heights = $derived(barHeights(points));
	const summary = $derived(seriesSummary(points, formatTimestamp, unit));
</script>

<figure
	class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4"
>
	<figcaption class="flex flex-col gap-1">
		<span class="text-sm font-medium">{title}</span>
		<span class="text-sm text-[var(--color-text-muted)]">{caption}</span>
	</figcaption>

	<div class="flex h-32 items-end gap-px" aria-hidden="true">
		{#each points as point, index (point.bucket)}
			<div
				class="min-w-px flex-1 rounded-t-[2px] bg-[var(--color-accent)]"
				style={`height: ${heights[index]}%`}
			></div>
		{/each}
	</div>

	<p class="text-sm">{summary}</p>

	{#if points.length > 0}
		<details class="text-sm">
			<summary class="min-h-11 cursor-pointer">Show these numbers as a table</summary>
			<div class="overflow-x-auto pt-2">
				<table class="w-full border-collapse">
					<caption class="sr-only">{title} by bucket</caption>
					<thead class="bg-[var(--color-surface-2)] text-left">
						<tr>
							<th scope="col" class="px-3 py-2 font-medium">Bucket</th>
							<th scope="col" class="px-3 py-2 font-medium">{title}</th>
						</tr>
					</thead>
					<tbody>
						{#each points as point (point.bucket)}
							<tr class="border-t border-[var(--color-border)]">
								<td class="px-3 py-2">{formatTimestamp(point.bucket)}</td>
								<td class="px-3 py-2 tabular-nums">{formatCount(point.value)}</td>
							</tr>
						{/each}
					</tbody>
				</table>
			</div>
		</details>
	{/if}
</figure>

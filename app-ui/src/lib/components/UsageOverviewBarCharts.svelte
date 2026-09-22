<script lang="ts">
	// The Overview's two bar charts, and the reads they need (docs/SPEC-UI/001-SPEC-UI.md §6.5, draft 016
	// F1, F2, F4).
	//
	// The reference draws a chart per dimension from a single stats read that carries every dimension at
	// once (`UsageStats.js:492-493`). This panel's summary answers for one dimension per call, so the pair
	// reads the two dimensions it draws and reuses the tab's own response when the operator's breakdown is
	// one of them. That is one extra read in the common case and two otherwise, and it is filed as an
	// app-serv request rather than hidden (draft 016 F4).
	//
	// A failed read is not a failed tab: each chart keeps its own state and says what could not be read
	// while the other still draws (R-27).
	//
	// The pair never reads `groups` inside its effect. That prop is replaced on every tab load, so reading
	// it there would make the charts re-read on every load; the effect keys on the window and the operator's
	// dimension, which are the values the reads depend on. The tab unmounts this component while it loads,
	// so a refresh remounts it and the reads run again with the new window.
	import UsageBarChart from '$lib/components/UsageBarChart.svelte';
	import { getUsageSummary } from '$lib/api/usage';
	import { usageQuery } from '$lib/schemas/usage-view';
	import { USAGE_MODEL_BAR_LIMIT, usageBars, type UsageMeasure } from '$lib/schemas/usage-bars';
	import type { UsageBreakdown, UsageGranularity, UsageGroup } from '$lib/schemas/usage';

	const DIMENSIONS = ['provider', 'model'] as const;
	type Dimension = (typeof DIMENSIONS)[number];

	type ChartState = { groups: UsageGroup[] | null; error: string | null };
	type Window = {
		from: string;
		to: string;
		granularity: UsageGranularity;
		groupBy: UsageBreakdown;
	};

	type Props = {
		/** The window the tab read, so both charts answer for the same period. */
		from: string;
		to: string;
		granularity: UsageGranularity;
		/** The dimension the tab's breakdown is showing, whose groups are reused instead of read again. */
		groupBy: UsageBreakdown;
		/** The tab's groups for `groupBy`, which are a chart's own data when the two dimensions match. */
		groups: UsageGroup[];
		/** Resolves a provider key to its registry name, or null while the registry has not answered. */
		providerNames: Map<string, string> | null;
	};

	let { from, to, granularity, groupBy, groups, providerNames }: Props = $props();

	let charts = $state<Record<Dimension, ChartState>>({
		provider: { groups: null, error: null },
		model: { groups: null, error: null }
	});
	let loading = $state(true);
	let providerMeasure = $state<UsageMeasure>('tokens');
	let modelMeasure = $state<UsageMeasure>('tokens');

	$effect(() => {
		void load({ from, to, granularity, groupBy });
	});

	async function load(window: Window): Promise<void> {
		const missing = DIMENSIONS.filter((dimension) => dimension !== window.groupBy);
		loading = true;

		const results = await Promise.all(
			missing.map((dimension) =>
				getUsageSummary(
					usageQuery({
						from: window.from,
						to: window.to,
						granularity: window.granularity,
						groupBy: dimension
					})
				)
			)
		);

		loading = false;

		const next: Record<Dimension, ChartState> = {
			provider: { groups: null, error: null },
			model: { groups: null, error: null }
		};

		missing.forEach((dimension, index) => {
			const result = results[index];
			next[dimension] = result.ok
				? { groups: result.data.groups, error: null }
				: {
						groups: null,
						error: `The ${dimension} breakdown could not be read (${result.error.message}).`
					};
		});

		charts = next;
	}

	function groupsFor(dimension: Dimension): UsageGroup[] {
		if (dimension === groupBy) return groups;

		return charts[dimension].groups ?? [];
	}

	function labelFor(dimension: Dimension): (key: string) => string {
		if (dimension === 'model') return (key) => key;

		return (key) => providerNames?.get(key) ?? key;
	}

	const providerSet = $derived(
		usageBars(groupsFor('provider'), { measure: providerMeasure, label: labelFor('provider') })
	);
	const modelSet = $derived(
		usageBars(groupsFor('model'), {
			measure: modelMeasure,
			label: labelFor('model'),
			limit: USAGE_MODEL_BAR_LIMIT
		})
	);
</script>

{#if loading}
	<p class="text-sm text-[var(--color-text-muted)]">Reading the provider and model breakdowns.</p>
{:else}
	<div class="grid min-w-0 gap-4 lg:grid-cols-2">
		<UsageBarChart
			title="By provider"
			caption="Usage per provider in this window, largest first. Tokens counts in plus out; cache tokens are in the tiles above and not here."
			noun="provider"
			set={providerSet}
			measure={providerMeasure}
			onmeasure={(measure) => (providerMeasure = measure)}
			error={charts.provider.error}
		/>
		<UsageBarChart
			title="Top models"
			caption="The models with the most usage in this window, five at most. Tokens counts in plus out; cache tokens are in the tiles above and not here."
			noun="model"
			set={modelSet}
			measure={modelMeasure}
			onmeasure={(measure) => (modelMeasure = measure)}
			error={charts.model.error}
		/>
	</div>
{/if}

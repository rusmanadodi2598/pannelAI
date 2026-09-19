<script lang="ts">
	// Usage (docs/SPEC-UI/001-SPEC-UI.md §6.5).
	//
	// The two tabs are one screen because they answer the same question at two levels of detail: what the
	// gateway consumed, and which requests consumed it. The Overview is the aggregate the API computes; the
	// Records tab is the rows behind it, and an operator moves from one to the other to answer "why is this
	// number what it is".
	import PanelTabs from '$lib/components/PanelTabs.svelte';
	import UsageOverviewTab from '$lib/components/UsageOverviewTab.svelte';
	import UsageRecordsTab from '$lib/components/UsageRecordsTab.svelte';

	const TABS = [
		{ id: 'overview', label: 'Overview' },
		{ id: 'records', label: 'Records' }
	];
</script>

<section class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Usage</h1>
		<p class="text-sm text-[var(--color-text-muted)]">
			What the gateway routed, what it consumed, and what it cost. Cost figures are estimates for
			display, not billing amounts.
		</p>
	</div>

	<PanelTabs tabs={TABS} label="Usage views">
		{#snippet panel(id)}
			{#if id === 'overview'}
				<UsageOverviewTab />
			{:else if id === 'records'}
				<UsageRecordsTab />
			{/if}
		{/snippet}
	</PanelTabs>
</section>

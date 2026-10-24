<script lang="ts">
	// Usage (docs/SPEC-UI/001-SPEC-UI.md §6.5).
	//
	// The two tabs are one screen because they answer the same question at two levels of detail: what the
	// gateway consumed, and which requests consumed it. The Overview is the aggregate the API computes; the
	// Records tab is the rows behind it, and an operator moves from one to the other to answer "why is this
	// number what it is".
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { SvelteURLSearchParams } from 'svelte/reactivity';
	import PanelTabs from '$lib/components/PanelTabs.svelte';
	import UsageOverviewTab from '$lib/components/UsageOverviewTab.svelte';
	import UsageRecordsTab from '$lib/components/UsageRecordsTab.svelte';

	const TABS = [
		{ id: 'overview', label: 'Overview' },
		{ id: 'records', label: 'Records' }
	];

	// The tab is URL state only so a breakdown row can open the rows behind it in one press. Overview is
	// the default and is left out of the URL rather than written on every visit.
	const tab = $derived(page.url.searchParams.get('tab') === 'records' ? 'records' : 'overview');

	function selectTab(id: string): void {
		const next = new SvelteURLSearchParams(page.url.searchParams);
		if (id === 'overview') next.delete('tab');
		else next.set('tab', id);

		const query = next.toString();
		void goto(resolve(query === '' ? '/usage' : `/usage?${query}`), {
			keepFocus: true,
			replaceState: true,
			noScroll: true
		});
	}
</script>

<section class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Usage</h1>
		<p class="text-sm text-[var(--color-text-muted)]">
			What the gateway routed, what it consumed, and what it cost.
		</p>
	</div>

	<PanelTabs tabs={TABS} label="Usage views" initial={tab} onchange={selectTab}>
		{#snippet panel(id)}
			{#if id === 'overview'}
				<UsageOverviewTab />
			{:else if id === 'records'}
				<UsageRecordsTab />
			{/if}
		{/snippet}
	</PanelTabs>
</section>

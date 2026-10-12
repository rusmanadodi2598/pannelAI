<script lang="ts">
	// The live detail of the Usage overview, in one tab component at the bottom of the panel.
	//
	// It holds the three facts the stream can state that a period window cannot: the requests in flight,
	// the provider the gateway last finished a request for, and the finished requests themselves. They sit
	// together so the page reads Node first and the detail after it, and each tab scrolls inside its own
	// box so the component stays compact however many rows a frame carries.
	//
	// The tabs are a statement, not a control of their data: the selection only chooses which list is shown.
	// The facts keep their colour rule (a routing provider's name in the status colour), and the finished
	// list keeps its own time, token and status rows.
	import PanelTabs from '$lib/components/PanelTabs.svelte';
	import UsageRecentList from '$lib/components/UsageRecentList.svelte';
	import type { LiveFact } from '$lib/schemas/usage-live-view';
	import type { UsageLiveActive, UsageLiveRecent } from '$lib/schemas/usage-live';

	type Props = {
		/** The facts the stream states, from `liveFacts`. Only the in-flight and finished facts appear here. */
		facts: LiveFact[];
		active: UsageLiveActive[];
		recent: UsageLiveRecent[];
		providerName: (id: string) => string;
	};

	let { facts, active, recent, providerName }: Props = $props();

	const inFlight = $derived(facts.find((fact) => fact.label.endsWith('in flight')) ?? null);
	const lastFinished = $derived(facts.find((fact) => fact.label === 'Last finished') ?? null);
	// The last error is a statement about the outcome of the last request, so it shares the Last finished
	// tab rather than taking a tab of its own: the stream still reports it, the way §6.5 says it does.
	const lastError = $derived(facts.find((fact) => fact.label === 'Last error') ?? null);

	// The tab labels say what the list holds. A count rides on the label when there is one, so the tab
	// itself reports how many rows sit behind it before it is opened.
	const TABS = $derived([
		{ id: 'in-flight', label: `In flight (${active.length})` },
		{ id: 'last-finished', label: 'Last finished' },
		{ id: 'finished', label: `Finished requests (${recent.length})` }
	]);

	// A scroll box per tab, the same height in every tab, so switching does not move the page.
	const SCROLL_BOX = 'max-h-48 overflow-y-auto rounded-[var(--radius-sm)] text-sm';
</script>

<PanelTabs tabs={TABS} label="Live activity">
	{#snippet panel(tab)}
		<div class={SCROLL_BOX}>
			{#if tab === 'in-flight'}
				{#if inFlight}
					<p class="break-words py-2">
						<span class="text-[var(--color-ok)]">{inFlight.value}</span>
					</p>
				{:else}
					<p class="py-2 text-[var(--color-text-muted)]">No request is in flight.</p>
				{/if}
			{:else if tab === 'last-finished'}
				{#if lastFinished}
					<p class="break-words py-2">{lastFinished.value}</p>
				{:else}
					<p class="py-2 text-[var(--color-text-muted)]">No request has finished yet.</p>
				{/if}
				{#if lastError}
					<p class="break-words py-2 text-[var(--color-danger)]">Last error: {lastError.value}</p>
				{/if}
			{:else if recent.length > 0}
				<UsageRecentList {recent} {providerName} />
			{:else}
				<p class="py-2 text-[var(--color-text-muted)]">No finished request in this stream.</p>
			{/if}
		</div>
	{/snippet}
</PanelTabs>

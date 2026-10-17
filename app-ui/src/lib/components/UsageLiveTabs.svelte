<script lang="ts">
	// The live detail of the Usage overview, in one tab component at the bottom of the panel.
	//
	// It holds the three facts the stream can state that a period window cannot: the requests in flight, the
	// provider the gateway last finished a request for, and the finished requests themselves. They sit in one
	// component so the page reads Node first and the detail after it, and the in-flight tab lists the
	// requests one row per request rather than as one joined sentence: the operator opening it is asking
	// which requests those are and how long each has been held, which a sentence cannot answer.
	//
	// The tabs are a statement, not a control of their data: the selection only chooses which list is shown.
	// The facts keep their colour rule (a routing provider's name in the status colour), and the finished
	// list keeps its own time, token and status rows.
	//
	// One box, one height, in every tab, and it scrolls its own overflow: the page never moves when the
	// stream fills the list, and switching tabs does not resize the region. The region wears the frame the
	// rest of this page's blocks wear (`rounded-[var(--radius-md)]` and one border, as the tables and the
	// totals tiles use it), because a scroll region with no edge is not readable as one region.
	import PanelTabs from '$lib/components/PanelTabs.svelte';
	import UsageRecentList from '$lib/components/UsageRecentList.svelte';
	import type { LiveFact } from '$lib/schemas/usage-live-view';
	import type { UsageLiveActive, UsageLiveRecent } from '$lib/schemas/usage-live';
	import { runningText } from '$lib/utils/time';

	type Props = {
		/** The facts the stream states, from `liveFacts`. Only the finished ones are drawn here; the in-flight rows carry the rest. */
		facts: LiveFact[];
		active: UsageLiveActive[];
		recent: UsageLiveRecent[];
		providerName: (id: string) => string;
		/** The panel's clock, so a running figure ages on the same tick that ages the staleness guard. */
		now: number;
	};

	let { facts, active, recent, providerName, now }: Props = $props();

	// The last error is a statement about the outcome of the last request, so it shares the Last finished
	// tab rather than taking a tab of its own: the stream still reports it, the way §6.5 says it does.
	const lastFinished = $derived(facts.find((fact) => fact.label === 'Last finished') ?? null);
	const lastError = $derived(facts.find((fact) => fact.label === 'Last error') ?? null);

	// The tab labels say what the list holds. A count rides on the label when there is one, so the tab
	// itself reports how many rows sit behind it before it is opened.
	const TABS = $derived([
		{ id: 'in-flight', label: `In flight (${active.length})` },
		{ id: 'last-finished', label: 'Last finished' },
		{ id: 'finished', label: `Finished requests (${recent.length})` }
	]);

	// The head of the in-flight list, named by the entry rather than by its position: the rows are keyed by
	// position, so a new arrival reuses the first row's node and only this string says it is now a different
	// request.
	const head = $derived(
		active.length === 0 ? '' : `${active[0].provider_id}|${active[0].started_at}`
	);

	// The frame the region wears, in the tokens this page already uses for a bounded block.
	const REGION_BOX = 'rounded-[var(--radius-md)] border border-[var(--color-border)] p-3';
	const SCROLL_BOX = 'h-48 overflow-y-auto text-sm';

	function comboOf(entry: UsageLiveActive): string {
		return entry.combo?.trim() ?? '';
	}

	function answeredBy(entry: UsageLiveActive): string {
		const name = providerName(entry.provider_id);
		const model = entry.model?.trim() ?? '';
		return model === '' ? name : `${name} (${model})`;
	}

	// The in-flight box follows a request that just started, because that row is the reason the tab is open.
	// The finished list is deliberately not followed: it is newest-first and read like a history, so a reader
	// scrolled into it is holding a position on purpose. One jump per arrival, gated on the frame, the same
	// way the drawing's motion is gated, and no loop of its own.
	function followNewest(node: HTMLElement, start: string | null) {
		let seen = start;
		return {
			update(next: string | null): void {
				if (next === null || next === seen) return;
				seen = next;
				node.scrollIntoView({ block: 'nearest' });
			}
		};
	}
</script>

<div class={REGION_BOX}>
	<PanelTabs tabs={TABS} label="Live activity">
		{#snippet panel(tab)}
			<div class={SCROLL_BOX}>
				{#if tab === 'in-flight'}
					{#if active.length > 0}
						<ul class="flex flex-col gap-1">
							{#each active as entry, index (index)}
								<li
									class="flex flex-wrap items-center gap-x-3 gap-y-1 py-1"
									use:followNewest={index === 0 ? head : null}
								>
									{#if comboOf(entry) !== ''}
										<span class="text-[var(--color-text-muted)]">{comboOf(entry)} →</span>
									{/if}
									<span class="truncate text-[var(--color-ok)]">{answeredBy(entry)}</span>
									<span class="text-[var(--color-text-muted)] tabular-nums"
										>{runningText(entry.started_at, now)}</span
									>
								</li>
							{/each}
						</ul>
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
						<p class="break-words py-2 text-[var(--color-danger)]">
							Last error: {lastError.value}
						</p>
					{/if}
				{:else if recent.length > 0}
					<UsageRecentList {recent} {providerName} />
				{:else}
					<p class="py-2 text-[var(--color-text-muted)]">No finished request in this stream.</p>
				{/if}
			</div>
		{/snippet}
	</PanelTabs>
</div>

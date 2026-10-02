<script lang="ts">
	// One quota card's body (docs/PORT/005-PORT-QUOTA-CARDS.md D1/D6; provider-first reshape 2026-10-02):
	// the sentence the provider-less lane opens with, then one group per connection whose FIRST thing is
	// what the provider itself reports, with this gateway's counted windows reduced to one summary line
	// beneath it. Split out of QuotaCards so the card (fold, checkbox, pagination) and the rows stay under
	// the file-size cap on their own; the scrolling wrapper stays in QuotaCards because `aria-controls`
	// points at it.
	//
	// The order is the whole point of the reshape. The operator opens this screen to read what each account
	// has left, and the gateway's own counters answer a different question — how much this proxy sent — so
	// they are stated once, compactly, instead of taking a second full row set over the same connection.
	// The two ledgers stay named and apart: neither is the other's correction (SPEC-API §7.12).
	//
	// The lane whose windows carry no provider gets no provider block, because there is no provider behind
	// it to have published anything; it keeps its own sentence and its counted summary.
	//
	// An account with no counted window is still a connection in `group.endpoints` (QuotaCards groups cards
	// from the union of the page's accounts, not from its windows): its provider block shows what the
	// provider published, or the "not polled yet" state, and its counted summary line says honestly that this
	// gateway holds no windows for it — `countedSummaryText([])` states the emptiness rather than a zero.
	import { countedSummaryText, type QuotaCardGroup } from '$lib/schemas/quota';
	import type { PublishedQuotaUsage } from '$lib/schemas/quota-published';
	import QuotaPublished from './QuotaPublished.svelte';

	type Props = {
		group: QuotaCardGroup;
		labels: Map<string, string>;
		/** The provider's answer per connection id. A connection absent from it is behind a provider
		 *  that publishes no quota, and gets no provider block at all. */
		published: Map<string, PublishedQuotaUsage>;
		now: number;
	};

	let { group, labels, published, now }: Props = $props();

	function endpointLabel(id: string): string {
		return labels.get(id) ?? id;
	}
</script>

{#if !group.provider}
	<p class="text-sm text-[var(--color-text-muted)]">
		Counted locally by this gateway. This lane has no provider quota.
	</p>
{/if}
{#each group.endpoints as endpoint (endpoint.id)}
	{@const answer = published.get(endpoint.id)}
	<div class="flex flex-col gap-2">
		<h4 class="truncate text-sm font-semibold">{endpointLabel(endpoint.id)}</h4>

		<!-- A connection with no answer on the page is behind a provider that publishes no quota: the
		     gateway sends an entry for every account it can ask, including one marked never-polled, so
		     silence here means there is nobody to ask. Offering the block anyway would print "not polled
		     yet" beside a provider that will never be polled, and a button that asks for nothing. -->
		{#if group.provider && answer !== undefined}
			<QuotaPublished
				endpointId={endpoint.id}
				provider={group.provider}
				label={endpointLabel(endpoint.id)}
				usage={answer}
				{now}
			/>
		{/if}

		<!-- The gateway's own count, in the one line it gets: how many windows it holds for this
		     connection and the largest spend among them. -->
		<p class="text-xs tabular-nums text-[var(--color-text-muted)]">
			{countedSummaryText(endpoint.windows, now)}
		</p>
	</div>
{/each}

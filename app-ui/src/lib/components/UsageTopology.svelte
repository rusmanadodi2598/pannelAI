<script lang="ts">
	// The live provider drawing's frame (docs/DRAFT/012-USAGE-LIVE-UI-READINESS.md F3, extended by
	// docs/DRAFT/013-USAGE-NODE-MOTION-READINESS.md F1 and F2, by docs/DRAFT/015 F1, and revised by
	// docs/DRAFT/023 F1).
	//
	// The reference fork draws this on a pan-and-zoom canvas (`ProviderTopology.js`). This is not that: the
	// panel has one screen for it, no dependency is worth adding for a dozen nodes, and a canvas an operator
	// can lose their place in is worse than a fixed drawing they can learn. The box itself lives in
	// `UsageTopologyDrawing.svelte`, which the beam pushed past the 220-line warning.
	//
	// The drawing is hidden from assistive technology, so the facts it encodes are stated in words above
	// it. Only the facts that are happening are stated: the provider list is what the node labels already
	// say, and an idle screen's "no request is in flight" repeats the absence of the colour, so neither is a
	// sentence that never changes (owner's correction, 2026-09-23).
	//
	// The paragraph that explained the drawing (which colour means what, what the beam does) is gone, and the
	// reference has no legend either (owner's correction, 2026-09-23, draft 023 F1). What replaces it is one
	// line of the facts, in the drawing's own colour rule: a routing provider's name takes the status colour,
	// exactly as its node's label does, and the finished and error facts name their provider without one,
	// exactly as those nodes' labels do. The line keeps one line's height while there is nothing to state,
	// so at the widths where the facts fit on one line a request starting does not move the drawing under
	// the reader's eyes (measured at 1360 px; at 390 px the stated line reads 60 px against the same 20 px
	// slot, and SPEC-UI §6.5 records both).
	//
	// Motion here is a state indicator, not decoration, and it has two conditions rather than one (draft 013
	// F2): the frame must name the provider, and frames must be arriving on the connection that is open now.
	// A stream the operator paused, or one that dropped, keeps the last known state on screen in colour
	// while every moving part stops, because motion is the one thing on this drawing that claims "happening
	// now" (R-36). Everything that moves is dropped for a reader who asked for reduced motion.
	import { resolve } from '$app/paths';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import UsageTopologyDrawing from '$lib/components/UsageTopologyDrawing.svelte';
	import type { UsageLiveActive } from '$lib/schemas/usage-live';
	import { topologyNodes } from '$lib/schemas/usage-topology-view';

	type Props = {
		/** The configured providers, which is what the drawing puts a node on. */
		providers: { id: string; name: string }[];
		/** The in-flight entries the guard still trusts. */
		active: UsageLiveActive[];
		/** The provider of the most recent completed request, or an empty string. */
		last: string;
		/** The provider the gateway last reported an error for, or an empty string. */
		error: string;
		/** Frames are arriving on the connection that is open now, so the drawing may move. */
		live: boolean;
	};

	let { providers, active, last, error, live }: Props = $props();

	const layout = $derived(
		topologyNodes(providers, {
			active: active.map((entry) => entry.provider_id),
			last,
			error
		})
	);

	/** A provider's display name, or its id when the registry has no entry for it. */
	function nameOf(id: string): string {
		const match = providers.find((provider) => provider.id.toLowerCase() === id.toLowerCase());
		return match?.name ?? id;
	}

	/** What is in flight, named with the model when the frame reported one. */
	function inFlight(entry: UsageLiveActive): string {
		const name = nameOf(entry.provider_id);
		return entry.model === undefined ? name : `${name} (${entry.model})`;
	}

	/**
	 * One live fact, in the drawing's own vocabulary: the label is the node's state, the value is the
	 * provider, and `tone` is the colour the drawing gives that state's name (only a routing node's label
	 * takes the status colour, so only this fact does).
	 */
	type Fact = { label: string; value: string; tone: string };

	/** The live facts that are happening, in the order the stream reports them, or an empty list. */
	const facts = $derived(
		[
			active.length === 0
				? null
				: {
						label: `${active.length} in flight`,
						value: active.map(inFlight).join(', '),
						tone: 'text-[var(--color-ok)]'
					},
			last === '' ? null : { label: 'Last finished', value: nameOf(last), tone: '' },
			error === '' ? null : { label: 'Last error', value: nameOf(error), tone: '' }
		].filter((fact): fact is Fact => fact !== null)
	);
</script>

<!-- eslint-disable svelte/no-useless-mustaches -- a fact reads as one sentence (`Label: value.`), and Svelte
     trims the whitespace between a mustache and the element beside it, so each `{' '}` is the space that
     sentence needs rather than a useless mustache (draft 023 F1). -->
<figure
	class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4"
>
	<!-- One line's height while there is nothing to state, so the drawing does not move when a request starts
	     wherever the facts fit on one line. `min-h-5` is the `text-sm` line box (draft 023 F1). -->
	<p class="min-h-5 text-sm">
		{#each facts as fact, index (fact.label)}{#if index > 0}{' '}{/if}<span
				class="text-[var(--color-text-muted)]">{fact.label}:</span
			>{' '}<span class={fact.tone}>{fact.value}.</span>{/each}
	</p>

	{#if providers.length === 0}
		<StateMessage
			kind="empty"
			title="No provider is configured"
			description="The drawing places one node per provider that has an endpoint or needs no credential. Nothing is configured yet, so there is nothing to route."
		>
			{#snippet action()}
				<a href={resolve('/providers')} class="underline">Open Providers</a>
			{/snippet}
		</StateMessage>
	{:else}
		<div class="mx-12 sm:mx-16">
			<UsageTopologyDrawing {layout} inFlight={active.length} {live} />
		</div>
	{/if}
</figure>

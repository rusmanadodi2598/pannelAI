<script lang="ts">
	// The live provider drawing's frame (docs/DRAFT/012-USAGE-LIVE-UI-READINESS.md F3, extended by
	// docs/DRAFT/013-USAGE-NODE-MOTION-READINESS.md F1 and F2, by docs/DRAFT/015 F1, and revised by
	// docs/DRAFT/023 F1 and 035 F2).
	//
	// The reference fork draws this on a pan-and-zoom canvas (`ProviderTopology.js`). This is not that: the
	// panel has one screen for it, no dependency is worth adding for a dozen nodes, and a canvas an operator
	// can lose their place in is worse than a fixed drawing they can learn. The box itself lives in
	// `UsageTopologyDrawing.svelte`, which the beam pushed past the 220-line warning.
	//
	// The drawing is hidden from assistive technology, and the live row above states in words the facts it
	// encodes. Those words used to sit inside this frame, above the nodes, and every request that started
	// or finished moved the drawing under the reader's eyes; the owner's correction of 2026-09-27 (draft
	// 035 F2) moved them into a tab of the row, and the frame carries the drawing alone. Only the facts that
	// are happening were ever stated, and that rule came along: the provider list is what the node labels
	// already say, and an idle screen's "no request is in flight" repeats the absence of the colour, so
	// neither is a sentence that never changes (owner's correction, 2026-09-23).
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
</script>

<figure
	class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4"
>
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

<script lang="ts">
	// The live provider drawing's frame (docs/DRAFT/012-USAGE-LIVE-UI-READINESS.md F3, extended by
	// docs/DRAFT/013-USAGE-NODE-MOTION-READINESS.md F1 and F2, and by docs/DRAFT/015 F1).
	//
	// The reference fork draws this on a pan-and-zoom canvas (`ProviderTopology.js`). This is not that: the
	// panel has one screen for it, no dependency is worth adding for a dozen nodes, and a canvas an operator
	// can lose their place in is worse than a fixed drawing they can learn. The box itself lives in
	// `UsageTopologyDrawing.svelte`, which the beam pushed past the 220-line warning.
	//
	// The drawing is hidden from assistive technology and the sentences below it carry the same facts,
	// which is the rule §6.5 sets for the chart and the reason this component has no `aria-label` prose of
	// its own.
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

	const listing = $derived(
		providers.length === 1
			? `One provider is configured: ${providers[0].name}.`
			: `${providers.length} providers are configured: ${providers.map((p) => p.name).join(', ')}.`
	);

	const running = $derived(
		active.length === 0
			? 'No request is in flight.'
			: `${active.length} in flight: ${active.map(inFlight).join(', ')}.`
	);

	const settled = $derived(
		last === ''
			? 'No request has finished since this screen opened.'
			: `The last request to finish went to ${nameOf(last)}.`
	);

	const reported = $derived(
		error === '' ? '' : ` The gateway last reported an error on ${nameOf(error)}.`
	);
</script>

<figure
	class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4"
>
	<figcaption class="text-sm text-[var(--color-text-muted)]">
		Every configured provider around the gateway. Green is routing now, amber finished last, red is
		where the gateway last reported an error. A routing provider's line carries a beam with dots
		running along it, its node a soft glow, and the gateway pulses while it counts the requests in
		flight.
	</figcaption>

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

	<!-- The same facts in words. Left out when there is no node to describe: the empty state above is the
	     statement for that case, and "0 providers are configured: ." is not a sentence. -->
	{#if providers.length > 0}
		<div class="flex flex-col gap-1 text-sm">
			<p>{listing}</p>
			<p>{running} {settled}{reported}</p>
		</div>
	{/if}
</figure>

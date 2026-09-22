<script lang="ts">
	// The live provider drawing (docs/DRAFT/012-USAGE-LIVE-UI-READINESS.md F3).
	//
	// The reference fork draws this on a pan-and-zoom canvas (`ProviderTopology.js`). This is not that: the
	// panel has one screen for it, no dependency is worth adding for a dozen nodes, and a canvas an operator
	// can lose their place in is worse than a fixed drawing they can learn.
	//
	// Everything positional is a percentage, so the same layout fills a phone and a desktop panel with no
	// measurement, no resize observer, and no arithmetic in the markup. The edges are one SVG stretched over
	// the same box with `preserveAspectRatio="none"`: a linear map sends a straight line to a straight line,
	// so a line drawn from the centre to a node's percentages ends exactly under that node, and
	// `non-scaling-stroke` keeps its weight from being stretched with it.
	//
	// The drawing is hidden from assistive technology and the sentences below it carry the same facts,
	// which is the rule §6.5 sets for the chart and the reason this component has no `aria-label` prose of
	// its own. The pulse is a state indicator, not decoration: it exists only while a provider has a request
	// in flight, and it is dropped entirely for a reader who asked for reduced motion.
	import { resolve } from '$app/paths';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import type { UsageLiveActive } from '$lib/schemas/usage-live';
	import { topologyNodes, type TopologyState } from '$lib/schemas/usage-topology-view';

	type Props = {
		/** The configured providers, which is what the drawing puts a node on. */
		providers: { id: string; name: string }[];
		/** The in-flight entries the guard still trusts. */
		active: UsageLiveActive[];
		/** The provider of the most recent completed request, or an empty string. */
		last: string;
		/** The provider the gateway last reported an error for, or an empty string. */
		error: string;
	};

	let { providers, active, last, error }: Props = $props();

	const layout = $derived(
		topologyNodes(providers, {
			active: active.map((entry) => entry.provider_id),
			last,
			error
		})
	);

	const DOT: Record<TopologyState, string> = {
		active: 'bg-[var(--color-ok)]',
		last: 'bg-[var(--color-warn)]',
		error: 'bg-[var(--color-danger)]',
		idle: 'bg-[var(--color-border)]'
	};

	const EDGE: Record<TopologyState, string> = {
		active: 'stroke-[var(--color-ok)] [stroke-width:2]',
		last: 'stroke-[var(--color-warn)] [stroke-width:1.5]',
		error: 'stroke-[var(--color-danger)] [stroke-width:2]',
		idle: 'stroke-[var(--color-border)] [stroke-width:1]'
	};

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
		where the gateway last reported an error.
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
			<div class="relative w-full" style={`height: ${layout.height}px`} aria-hidden="true">
				<svg class="absolute inset-0 size-full" viewBox="0 0 100 100" preserveAspectRatio="none">
					{#each layout.nodes as node (node.id)}
						<line
							x1="50"
							y1="50"
							x2={node.x}
							y2={node.y}
							class={EDGE[node.state]}
							vector-effect="non-scaling-stroke"
						/>
					{/each}
				</svg>

				<div
					class="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 rounded-[var(--radius-sm)] border border-[var(--color-accent)] bg-[var(--color-surface)] px-3 py-2 text-sm font-medium whitespace-nowrap"
				>
					Gateway
				</div>

				{#each layout.nodes as node (node.id)}
					<div
						class="absolute flex -translate-x-1/2 -translate-y-1/2 items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 py-1"
						style={`left: ${node.x}%; top: ${node.y}%`}
					>
						<span class="relative flex size-2 shrink-0">
							{#if node.state === 'active'}
								<span
									class="absolute inline-flex size-full animate-ping rounded-full bg-[var(--color-ok)] opacity-75 motion-reduce:hidden"
								></span>
							{/if}
							<span class={`relative inline-flex size-2 rounded-full ${DOT[node.state]}`}></span>
						</span>
						<span class="max-w-24 truncate text-sm">{node.name}</span>
					</div>
				{/each}
			</div>
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

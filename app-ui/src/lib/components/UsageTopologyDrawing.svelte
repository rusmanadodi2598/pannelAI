<script lang="ts">
	// The live drawing's box (docs/DRAFT/012 F3, extended by draft 013 F1/F2 and draft 015 F1).
	//
	// Split from `UsageTopology.svelte` when the beam arrived: the frame around the drawing (caption, empty
	// state, the sentences that carry the same facts) and the drawing itself crossed the 220-line warning
	// together, and the seam is real rather than arithmetic. The drawing is still hidden from assistive
	// technology, so every fact it encodes is stated by the frame's sentences.
	//
	// Three parts, and the middle one is its own file: this box and the unit it publishes, the edges
	// (`UsageTopologyEdges.svelte`, split out by draft 018 when the beam's constants and these comments took
	// this file past the 220-line warning), and the cards: the gateway and one box per node.
	//
	// Everything positional is a percentage, so the same layout fills a phone and a desktop panel with no
	// measurement and no resize observer. The box's own width is the one thing the drawing does read, through
	// the container unit `cqw` rather than through JavaScript: every metric below is written as its own pixel
	// value times `--u`, and `--u` is the box's width over the width at which a node reaches its cap. A node's
	// share of the box comes from `topologyNodes`, so the drawing shrinks with a narrow box exactly as the
	// reference fork's fitView shrinks its whole canvas, and stops shrinking once a node is as wide as the
	// panel ever draws one. The node boxes, the gateway and the beam's strokes all take the same unit, so a
	// phone draws a smaller drawing rather than a collided one (draft 018 F1/F2). The one metric that does not
	// scale is the 1px box border: a hairline is the panel's token for an edge, and below a pixel it would not
	// be drawn at all.
	//
	// Every metric is written on the element that uses it, and that is not a style choice: an element is not
	// its own query container, so a metric declared on the drawing box itself resolves `cqw` against whatever
	// contains the drawing. The first cut put the font size there, and the browser measured the result: 8.4px
	// text inside 47px nodes at 390px, because the text resolved against the page while the padding resolved
	// against the drawing. Both are on the node and gateway boxes now, and both resolve against the drawing.
	import UsageTopologyEdges from './UsageTopologyEdges.svelte';
	import {
		NODE_MAX_WIDTH,
		type TopologyLayout,
		type TopologyState
	} from '$lib/schemas/usage-topology-view';

	type Props = {
		/** Where the nodes go and how tall the box is. */
		layout: TopologyLayout;
		/** The requests in flight, which is what the gateway counts. */
		inFlight: number;
		/** Frames are arriving on the connection that is open now, so the drawing may move. */
		live: boolean;
	};

	let { layout, inFlight, live }: Props = $props();

	// Per state: the dot's fill, the node's own border, and the label's colour. Active is the only state
	// that takes the status colour and the reference's soft glow (`ProviderTopology.js:41-42`, the same
	// 16px radius at a quarter alpha), and it keeps them while the stream is down: colour is state, motion
	// is not.
	const STATE: Record<TopologyState, { dot: string; node: string; label: string }> = {
		active: {
			dot: 'bg-[var(--color-ok)]',
			node: 'border-[var(--color-ok)] shadow-[0_0_16px_color-mix(in_srgb,var(--color-ok)_25%,transparent)]',
			label: 'text-[var(--color-ok)]'
		},
		last: { dot: 'bg-[var(--color-warn)]', node: 'border-[var(--color-border)]', label: '' },
		error: { dot: 'bg-[var(--color-danger)]', node: 'border-[var(--color-border)]', label: '' },
		idle: { dot: 'bg-[var(--color-border)]', node: 'border-[var(--color-border)]', label: '' }
	};
</script>

<div
	class="relative w-full [container-type:inline-size]"
	style={`height: ${layout.height}px; --share: ${layout.nodeShare}; --u: min(1, calc(var(--share) * tan(atan2(100cqw, ${NODE_MAX_WIDTH}px)))); line-height: 1.4286`}
	aria-hidden="true"
>
	<UsageTopologyEdges nodes={layout.nodes} {live} />

	<!-- The gateway, which is the reference's router node (`ProviderTopology.js:99-130`): while it is
	     routing, the card pulses, the mark shakes, the label flickers and the count sits in a glowing
	     badge (`globals.css:513-524`, the same 0.75s, 0.45s and 0.7s cycles). Each of the four keeps one
	     glow layer in the status colour instead of the reference's four-layer neon stack, which is the
	     dose cap R-13 asks for. -->
	<div class="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2">
		<div
			class={`flex items-center gap-[calc(8px*var(--u))] rounded-[var(--radius-sm)] border bg-[var(--color-surface)] px-[calc(12px*var(--u))] py-[calc(8px*var(--u))] font-medium whitespace-nowrap [font-size:calc(14px*var(--u))] ${
				inFlight > 0 ? 'border-[var(--color-ok)]' : 'border-[var(--color-accent)]'
			} ${inFlight > 0 && live ? 'animate-router-pulse motion-reduce:animate-none' : ''}`}
		>
			<img
				src="/logo-mark@128.png"
				alt=""
				class={`size-[calc(16px*var(--u))] shrink-0 ${inFlight > 0 && live ? 'animate-router-shake motion-reduce:animate-none' : ''}`}
			/>
			<span class={inFlight > 0 && live ? 'animate-router-flicker motion-reduce:animate-none' : ''}
				>Gateway</span
			>
			{#if inFlight > 0}
				<span
					class="inline-flex min-w-[calc(20px*var(--u))] justify-center rounded-[var(--radius-sm)] bg-[var(--color-ok)] px-[calc(4px*var(--u))] text-[0.857em] font-semibold text-[var(--color-surface)] shadow-[0_0_10px_color-mix(in_srgb,var(--color-ok)_45%,transparent)]"
					>{inFlight}</span
				>
			{/if}
		</div>
	</div>

	{#each layout.nodes as node (node.id)}
		<div
			class={`absolute flex -translate-x-1/2 -translate-y-1/2 items-center gap-[calc(8px*var(--u))] rounded-[var(--radius-sm)] border bg-[var(--color-surface)] px-[calc(8px*var(--u))] py-[calc(4px*var(--u))] transition-all duration-300 [font-size:calc(14px*var(--u))] ${
				STATE[node.state].node
			}`}
			style={`left: ${node.x}%; top: ${node.y}%`}
		>
			<span class="relative flex size-[calc(8px*var(--u))] shrink-0">
				{#if node.state === 'active' && live}
					<span
						class="absolute inline-flex size-full animate-ping rounded-full bg-[var(--color-ok)] opacity-75 motion-reduce:hidden"
					></span>
				{/if}
				<span class={`relative inline-flex size-full rounded-full ${STATE[node.state].dot}`}></span>
			</span>
			<span class={`max-w-[calc(96px*var(--u))] truncate ${STATE[node.state].label}`}
				>{node.name}</span
			>
		</div>
	{/each}
</div>

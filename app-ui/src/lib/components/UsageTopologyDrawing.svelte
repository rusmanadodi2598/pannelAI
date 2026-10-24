<script lang="ts">
	// The live drawing's box.
	//
	// Split from `UsageTopology.svelte` on a real seam rather than an arithmetic one: the frame around the
	// drawing (caption and empty state) is one job, and the drawing is another. The drawing is hidden from
	// assistive technology; the facts it encodes are stated in words by the live panel's header.
	//
	// Three parts, and the middle one is its own file: this box and the cards it publishes (the gateway,
	// one card per node on each band, and the two terminals on the vertical axis), the hops
	// (`UsageTopologyEdges.svelte`), and the beam each lit hop carries (`UsageTopologyHop.svelte`).
	//
	// Everything positional is a percentage, so the same layout fills a phone and a desktop panel with no
	// measurement and no resize observer.
	//
	// The box's own width is the one thing the drawing does read, through the container unit `cqw` rather
	// than through JavaScript: every metric below is written as its own pixel value times `--u`, and `--u` is
	// the box's width over the width at which a node reaches its cap. A node's share of the box comes from
	// `topologyNodes`, so the drawing shrinks with a narrow box exactly as the reference fork's fitView
	// shrinks its whole canvas, and stops shrinking once a node is as wide as the panel ever draws one. The
	// node boxes, the gateway and the beam's strokes all take the same unit, so a phone draws a smaller
	// drawing rather than a collided one. The one metric that does not scale is the 1px box
	// border: a hairline is the panel's token for an edge, and below a pixel it would not be drawn at all.
	//
	// Every metric is written on the element that uses it, and that is not a style choice: an element is not
	// its own query container, so a metric declared on the drawing box itself resolves `cqw` against whatever
	// contains the drawing. The first cut put the font size there, and the browser measured the result: 8.4px
	// text inside 47px nodes at 390px, because the text resolved against the page while the padding resolved
	// against the drawing. Both are on the node and gateway boxes now, and both resolve against the drawing.
	//
	// Motion belongs to the live state alone, and the hub is the only focal mover. The two things that move
	// on entry (a card settling in when it appears, and the box growing when the node count changes) are not
	// that motion: they say "this drawing is the same drawing, with one more thing on it", and both stop for
	// a reader who asked for reduced motion.
	import UsageTopologyEdges from './UsageTopologyEdges.svelte';
	import {
		CLIENT_POSITION,
		NODE_MAX_WIDTH,
		RESPONSE_POSITION
	} from '$lib/schemas/usage-topology-geometry';
	import type { TopologyLayout, TopologyState } from '$lib/schemas/usage-topology-view';

	type Props = {
		/** Where the nodes go, where the terminals go, and how tall the box is. */
		layout: TopologyLayout;
		/** The requests in flight, which is what the gateway counts. */
		inFlight: number;
		/** Frames are arriving on the connection that is open now, so the drawing may move. */
		live: boolean;
	};

	let { layout, inFlight, live }: Props = $props();

	// Per state: the dot's fill, the node's own border, the label's colour, and the 3px leading marker.
	// Active is the only state that takes the status colour on the border and the label, and it keeps them
	// while the stream is down: colour is state, motion is not. The marker repeats the panel's one identity
	// motif, and it never stands alone because the label and dot carry the same state.
	const STATE: Record<TopologyState, { dot: string; node: string; label: string; marker: string }> =
		{
			active: {
				dot: 'bg-[var(--color-ok)]',
				node: 'border-[var(--color-ok)]',
				label: 'text-[var(--color-ok)]',
				marker: 'bg-[var(--color-ok)]'
			},
			last: {
				dot: 'bg-[var(--color-warn)]',
				node: 'border-[var(--color-border)]',
				label: '',
				marker: 'bg-[var(--color-warn)]'
			},
			error: {
				dot: 'bg-[var(--color-danger)]',
				node: 'border-[var(--color-border)]',
				label: '',
				marker: 'bg-[var(--color-danger)]'
			},
			idle: {
				dot: 'bg-[var(--color-border)]',
				node: 'border-[var(--color-border)]',
				label: '',
				marker: 'bg-[var(--color-border)]'
			}
		};

	// The terminals take the same state colours as a node, without the glow or a loop: a terminal is lit by
	// what is crossing it, and it never stands for one provider that is being called. They are pills rather
	// than boxes, and they carry no state dot, so they read as the ends of the path rather than as two more
	// entries in it.
	function terminalClass(state: TopologyState): string {
		if (state === 'active') return 'border-[var(--color-ok)] text-[var(--color-ok)]';
		if (state === 'last') return 'border-[var(--color-warn)]';
		if (state === 'error') return 'border-[var(--color-danger)]';
		return 'border-[var(--color-border)]';
	}

	const terminals = $derived([
		{ label: 'Client', position: CLIENT_POSITION, state: layout.client.state },
		{ label: 'Response', position: RESPONSE_POSITION, state: layout.response.state }
	]);
</script>

<div
	class="relative w-full transition-[height] duration-300 [container-type:inline-size] motion-reduce:transition-none"
	style={`height: ${layout.height}px; --share: ${layout.nodeShare}; --u: min(1, calc(var(--share) * tan(atan2(100cqw, ${NODE_MAX_WIDTH}px)))); line-height: 1.4286`}
	aria-hidden="true"
>
	<UsageTopologyEdges {layout} {live} />

	<!-- The gateway, which is the reference's router node (`ProviderTopology.js`): while it is routing, the
	     card pulses, the mark breathes, the label eases and the count sits in a badge. The ring is the one
	     loop and the one glow the drawing keeps; the badge itself is matte. -->
	<div class="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2">
		<div
			class={`flex items-center gap-[calc(8px*var(--u))] rounded-[var(--radius-sm)] border bg-[var(--color-surface)] px-[calc(12px*var(--u))] py-[calc(8px*var(--u))] font-medium whitespace-nowrap [font-size:max(12px,calc(14px*var(--u)))] animate-node-enter motion-reduce:animate-none ${
				inFlight > 0 ? 'border-[var(--color-ok)]' : 'border-[var(--color-border)]'
			} ${inFlight > 0 && live ? 'animate-router-pulse motion-reduce:animate-none' : ''}`}
		>
			<img
				src="/logo-mark@128.svg"
				alt=""
				class={`size-[max(14px,calc(16px*var(--u)))] shrink-0 ${inFlight > 0 && live ? 'animate-router-shake motion-reduce:animate-none' : ''}`}
			/>
			<span class={inFlight > 0 && live ? 'animate-router-flicker motion-reduce:animate-none' : ''}
				>Gateway</span
			>
			{#if inFlight > 0}
				<span
					class="inline-flex min-w-[max(20px,calc(20px*var(--u)))] justify-center rounded-[var(--radius-sm)] bg-[var(--color-ok)] px-[calc(4px*var(--u))] text-[max(11px,0.857em)] font-semibold text-[var(--color-surface)]"
					>{inFlight}</span
				>
			{/if}
		</div>
	</div>

	<!-- The two ends of the path. They are pills rather than boxes so they read as terminals at a glance:
	     no state dot, nothing counted, and a name the drawing never takes from the wire. -->
	{#each terminals as terminal (terminal.label)}
		<div
			class="absolute -translate-x-1/2 -translate-y-1/2"
			style={`left: ${terminal.position.x}%; top: ${terminal.position.y}%`}
		>
			<div
				class={`flex items-center rounded-full border bg-[var(--color-surface)] px-[calc(10px*var(--u))] py-[calc(4px*var(--u))] font-medium whitespace-nowrap [font-size:max(11px,calc(13px*var(--u)))] animate-node-enter motion-reduce:animate-none ${terminalClass(terminal.state)}`}
			>
				{terminal.label}
			</div>
		</div>
	{/each}

	{#each layout.nodes as node (node.key)}
		<div
			class={`absolute flex -translate-x-1/2 -translate-y-1/2 animate-node-enter items-center gap-[calc(8px*var(--u))] rounded-[var(--radius-sm)] border bg-[var(--color-surface)] px-[calc(8px*var(--u))] py-[calc(4px*var(--u))] transition-all duration-300 [font-size:max(12px,calc(14px*var(--u)))] motion-reduce:animate-none ${
				STATE[node.state].node
			}`}
			style={`left: ${node.x}%; top: ${node.y}%`}
		>
			<span class={`h-5 w-[3px] shrink-0 rounded-[var(--radius-full)] ${STATE[node.state].marker}`}
			></span>
			<span class="relative flex size-[8px] shrink-0">
				<span class={`relative inline-flex size-full rounded-full ${STATE[node.state].dot}`}></span>
			</span>
			<span
				class={`max-w-[calc(96px*var(--u))] truncate ${STATE[node.state].label}`}
				title={node.name}>{node.name}</span
			>
		</div>
	{/each}
</div>

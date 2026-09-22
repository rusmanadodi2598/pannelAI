<script lang="ts">
	// The live drawing's box (docs/DRAFT/012 F3, extended by draft 013 F1/F2 and draft 015 F1).
	//
	// Split from `UsageTopology.svelte` when the beam arrived: the frame around the drawing (caption, empty
	// state, the sentences that carry the same facts) and the drawing itself crossed the 220-line warning
	// together, and the seam is real rather than arithmetic. The drawing is still hidden from assistive
	// technology, so every fact it encodes is stated by the frame's sentences.
	//
	// Everything positional is a percentage, so the same layout fills a phone and a desktop panel with no
	// measurement and no resize observer. The edges are one SVG stretched over the box with
	// `preserveAspectRatio="none"`: a linear map sends a straight line to a straight line, so a line drawn
	// from the centre to a node's percentages ends exactly under that node, and `non-scaling-stroke` keeps
	// its weight from being stretched with it.
	//
	// A routing edge is the reference fork's beam, in the panel's tokens: a wide halo, a dashed plasma, and
	// a dashed core, with six orbs and five sparks travelling along the same line (`ProviderTopology.js:137-245`
	// on `origin/master`). The dots are dashes with round caps rather than circles, because a circle in this
	// stretched box would be an ellipse of a size that depends on the box: a zero-length dash paints a dot
	// whose diameter is the stroke width, which `non-scaling-stroke` holds constant. Every moving part is
	// gated twice, on the state and on frames arriving, and dropped for a reader who asked for reduced motion.
	import {
		BEAM_CORE_DASH,
		BEAM_PARTICLE_DASH,
		BEAM_PLASMA_DASH,
		beamOrbs,
		beamSparks,
		type BeamTone
	} from '$lib/schemas/usage-beam';
	import type { TopologyLayout, TopologyState } from '$lib/schemas/usage-topology-view';

	type Props = {
		/** Where the nodes go and how tall the box is. */
		layout: TopologyLayout;
		/** The requests in flight, which is what the gateway counts. */
		inFlight: number;
		/** Frames are arriving on the connection that is open now, so the drawing may move. */
		live: boolean;
	};

	let { layout, inFlight, live }: Props = $props();

	const orbs = beamOrbs();
	const sparks = beamSparks();

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

	const EDGE: Record<TopologyState, string> = {
		active: 'stroke-[var(--color-ok)] [stroke-width:2]',
		last: 'stroke-[var(--color-warn)] [stroke-width:1.5]',
		error: 'stroke-[var(--color-danger)] [stroke-width:2]',
		idle: 'stroke-[var(--color-border)] [stroke-width:1]'
	};

	/** The three strokes of a routing edge, widest and faintest first. */
	const HALO =
		'stroke-[var(--color-ok)] [stroke-opacity:0.35] [stroke-width:10] animate-beam-halo motion-reduce:animate-none';
	const PLASMA =
		'stroke-[var(--color-ok)] [stroke-opacity:0.85] [stroke-width:5] animate-beam-plasma motion-reduce:animate-none';
	const CORE =
		'stroke-[var(--color-text)] [stroke-width:2.2] animate-beam-core motion-reduce:animate-none';

	/** A dot's travel, and the sparks' extra blink. `motion-reduce:hidden` because a dot has no rest state. */
	const TRAVEL = 'animate-beam-travel motion-reduce:hidden';
	const SPARK = 'animate-beam-spark motion-reduce:hidden';

	const TONE: Record<BeamTone, string> = {
		ok: 'stroke-[var(--color-ok)]',
		text: 'stroke-[var(--color-text)]'
	};

	/** The dot's own numbers, with the shared `stroke-linecap` that turns a dash into a dot. */
	const DOT = '[stroke-linecap:round]';
</script>

<div class="relative w-full" style={`height: ${layout.height}px`} aria-hidden="true">
	<svg class="absolute inset-0 size-full" viewBox="0 0 100 100" preserveAspectRatio="none">
		{#each layout.nodes as node (node.id)}
			{@const geometry = {
				x1: 50,
				y1: 50,
				x2: node.x,
				y2: node.y,
				pathLength: 100,
				'vector-effect': 'non-scaling-stroke'
			}}
			{#if node.state === 'active' && live}
				<!-- The reference's turbulence (`ProviderTopology.js:167-172`: baseFrequency 0.9, two
				     octaves, seed 2), held still. It animates that frequency there with SMIL, which
				     cannot honour a reduced-motion preference, so the wobble is a texture here and the
				     motion comes from the dashes. The displacement is 0.4 rather than the reference's
				     3.5 because this path lives in a 100-unit box instead of pixel coordinates. -->
				<defs>
					<filter
						id={`beam-${node.id}`}
						filterUnits="userSpaceOnUse"
						x="0"
						y="0"
						width="100"
						height="100"
					>
						<feTurbulence
							type="fractalNoise"
							baseFrequency="0.9"
							numOctaves="2"
							seed="2"
							result="noise"
						/>
						<feDisplacementMap
							in="SourceGraphic"
							in2="noise"
							scale="0.4"
							xChannelSelector="R"
							yChannelSelector="G"
						/>
					</filter>
				</defs>
				<line {...geometry} data-beam="halo" class={HALO} filter={`url(#beam-${node.id})`} />
				<line
					{...geometry}
					data-beam="plasma"
					class={PLASMA}
					style={`stroke-dasharray:${BEAM_PLASMA_DASH}`}
				/>
				<line
					{...geometry}
					data-beam="core"
					class={CORE}
					style={`stroke-dasharray:${BEAM_CORE_DASH}`}
				/>
				{#each orbs as orb (orb.key)}
					<line
						{...geometry}
						data-beam="orb"
						class={`${DOT} ${TONE[orb.tone]} ${TRAVEL}`}
						style={`stroke-width:${orb.width}; stroke-dasharray:${BEAM_PARTICLE_DASH}; animation-duration:${orb.travel}s; animation-delay:${orb.delay}s`}
					/>
				{/each}
				{#each sparks as spark (spark.key)}
					<line
						{...geometry}
						data-beam="spark"
						class={`${DOT} ${TONE[spark.tone]} ${SPARK}`}
						style={`stroke-width:${spark.width}; stroke-dasharray:${BEAM_PARTICLE_DASH}; animation-duration:${spark.travel}s, ${spark.blink}s; animation-delay:${spark.delay}s, 0s`}
					/>
				{/each}
			{:else}
				<line {...geometry} class={EDGE[node.state]} />
			{/if}
		{/each}
	</svg>

	<!-- The gateway, which is the reference's router node (`ProviderTopology.js:99-130`): while it is
	     routing, the card pulses, the mark shakes, the label flickers and the count sits in a glowing
	     badge (`globals.css:513-524`, the same 0.75s, 0.45s and 0.7s cycles). Each of the four keeps one
	     glow layer in the status colour instead of the reference's four-layer neon stack, which is the
	     dose cap R-13 asks for. -->
	<div class="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2">
		<div
			class={`flex items-center gap-2 rounded-[var(--radius-sm)] border bg-[var(--color-surface)] px-3 py-2 text-sm font-medium whitespace-nowrap ${
				inFlight > 0 ? 'border-[var(--color-ok)]' : 'border-[var(--color-accent)]'
			} ${inFlight > 0 && live ? 'animate-router-pulse motion-reduce:animate-none' : ''}`}
		>
			<img
				src="/logo-mark@128.png"
				alt=""
				class={`size-4 shrink-0 ${inFlight > 0 && live ? 'animate-router-shake motion-reduce:animate-none' : ''}`}
			/>
			<span class={inFlight > 0 && live ? 'animate-router-flicker motion-reduce:animate-none' : ''}
				>Gateway</span
			>
			{#if inFlight > 0}
				<span
					class="inline-flex min-w-5 justify-center rounded-[var(--radius-sm)] bg-[var(--color-ok)] px-1 text-xs font-semibold text-[var(--color-surface)] shadow-[0_0_10px_color-mix(in_srgb,var(--color-ok)_45%,transparent)]"
					>{inFlight}</span
				>
			{/if}
		</div>
	</div>

	{#each layout.nodes as node (node.id)}
		<div
			class={`absolute flex -translate-x-1/2 -translate-y-1/2 items-center gap-2 rounded-[var(--radius-sm)] border bg-[var(--color-surface)] px-2 py-1 transition-all duration-300 ${
				STATE[node.state].node
			}`}
			style={`left: ${node.x}%; top: ${node.y}%`}
		>
			<span class="relative flex size-2 shrink-0">
				{#if node.state === 'active' && live}
					<span
						class="absolute inline-flex size-full animate-ping rounded-full bg-[var(--color-ok)] opacity-75 motion-reduce:hidden"
					></span>
				{/if}
				<span class={`relative inline-flex size-2 rounded-full ${STATE[node.state].dot}`}></span>
			</span>
			<span class={`max-w-24 truncate text-sm ${STATE[node.state].label}`}>{node.name}</span>
		</div>
	{/each}
</div>

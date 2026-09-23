<script lang="ts">
	// The live drawing's edges (docs/DRAFT/012 F3; the beam is draft 015 F1, and the split is draft 018).
	//
	// Split from `UsageTopologyDrawing.svelte` when the beam's own constants and draft 018's comments took
	// that file past the 220-line warning. The seam is the layer: one SVG stretched over the box, one edge
	// per node, and the reference's beam along the edge of a provider that is routing right now. The drawing
	// keeps the box's cards, which are a different technique: absolutely positioned HTML sized by the
	// container unit, where these are lines in a stretched viewBox.
	//
	// The unit `--u` is the drawing box's own. It is set on the drawing's root, inherited here, and every
	// width below is written as its own pixel value times it, so the beam shrinks with a narrow box exactly
	// as the node boxes do (draft 018 F2).
	//
	// The lines are drawn in the box's own units: `viewBox="0 0 100 100"` with `preserveAspectRatio="none"`
	// stretches the box, and a linear map sends a straight line to a straight line, so a line drawn from the
	// centre to a node's percentages ends exactly under that node. `non-scaling-stroke` keeps the stroke
	// weight from being stretched with it.
	//
	// A routing edge is the reference fork's beam (`ProviderTopology.js:137-245` on `origin/master`): a wide
	// halo, a dashed plasma and a dashed core, with six orbs and five sparks travelling along the same line.
	// The dots are dashes with round caps rather than circles, because a circle in this stretched box would
	// be an ellipse of a size that depends on the box: a zero-length dash paints a dot whose diameter is the
	// stroke width, which `non-scaling-stroke` holds constant. Every moving part is gated twice, on the state
	// and on frames arriving, and dropped for a reader who asked for reduced motion.
	import {
		BEAM_CORE_DASH,
		BEAM_PARTICLE_DASH,
		BEAM_PLASMA_DASH,
		beamOrbs,
		beamSparks,
		type BeamTone
	} from '$lib/schemas/usage-beam';
	import type { TopologyNode, TopologyState } from '$lib/schemas/usage-topology-view';

	type Props = {
		/** Where each edge ends, which is where the drawing puts that node's box. */
		nodes: TopologyNode[];
		/** Frames are arriving on the connection that is open now, so the beam may move. */
		live: boolean;
	};

	let { nodes, live }: Props = $props();

	const orbs = beamOrbs();
	const sparks = beamSparks();

	/** The one line an edge that is not carrying a beam is drawn as. */
	const EDGE: Record<TopologyState, string> = {
		active: 'stroke-[var(--color-ok)] [stroke-width:2]',
		last: 'stroke-[var(--color-warn)] [stroke-width:1.5]',
		error: 'stroke-[var(--color-danger)] [stroke-width:2]',
		idle: 'stroke-[var(--color-border)] [stroke-width:1]'
	};

	/** The three strokes of a routing edge, widest and faintest first. */
	const HALO =
		'stroke-[var(--color-ok)] [stroke-opacity:0.35] [stroke-width:calc(10px*var(--u))] animate-beam-halo motion-reduce:animate-none';
	const PLASMA =
		'stroke-[var(--color-ok)] [stroke-opacity:0.85] [stroke-width:calc(5px*var(--u))] animate-beam-plasma motion-reduce:animate-none';
	const CORE =
		'stroke-[var(--color-text)] [stroke-width:calc(2.2px*var(--u))] animate-beam-core motion-reduce:animate-none';

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

<svg class="absolute inset-0 size-full" viewBox="0 0 100 100" preserveAspectRatio="none">
	{#each nodes as node (node.id)}
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
					style={`stroke-width:calc(${orb.width}px*var(--u)); stroke-dasharray:${BEAM_PARTICLE_DASH}; animation-duration:${orb.travel}s; animation-delay:${orb.delay}s`}
				/>
			{/each}
			{#each sparks as spark (spark.key)}
				<line
					{...geometry}
					data-beam="spark"
					class={`${DOT} ${TONE[spark.tone]} ${SPARK}`}
					style={`stroke-width:calc(${spark.width}px*var(--u)); stroke-dasharray:${BEAM_PARTICLE_DASH}; animation-duration:${spark.travel}s, ${spark.blink}s; animation-delay:${spark.delay}s, 0s`}
				/>
			{/each}
		{:else}
			<line {...geometry} class={EDGE[node.state]} />
		{/if}
	{/each}
</svg>

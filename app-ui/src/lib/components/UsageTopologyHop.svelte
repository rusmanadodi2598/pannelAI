<script lang="ts">
	// One hop of the live drawing's request path (docs/DRAFT/012 F3; the beam is draft 015 F1, the split
	// from the edge list is docs/DRAFT/043 F2).
	//
	// A hop is drawn one of two ways and never both: one line, in the state's own weight and colour, while
	// nothing is travelling it; the reference fork's beam (`ProviderTopology.js:137-245`) while a request is
	// moving — a wide halo, a dashed plasma and a dashed core, with six orbs and five sparks along the same
	// line. The dots are dashes with round caps rather than circles, because a circle in this stretched box
	// would be an ellipse of a size that depends on the box: a zero-length dash paints a dot whose diameter
	// is the stroke width, which `non-scaling-stroke` holds constant. Every moving part is gated twice, on
	// the state and on frames arriving, and dropped for a reader who asked for reduced motion.
	//
	// Direction is the endpoint order, not an animation: `beam-dash` moves the dash offset by -100 along a
	// path normalized to `pathLength="100"`, which carries every dash from the line's start to its end. So
	// the message's own direction is `from` then `to`, and a hop that reads backwards on screen is drawn
	// backwards here rather than given a second keyframe.
	//
	// The unit `--u` is the drawing box's own. It is set on the drawing's root, inherited here, and every
	// width below is written as its own pixel value times it, so the beam shrinks with a narrow box exactly
	// as the node boxes do (draft 018 F2).
	import {
		BEAM_CORE_DASH,
		BEAM_PARTICLE_DASH,
		BEAM_PLASMA_DASH,
		beamOrbs,
		beamSparks,
		type BeamTone
	} from '$lib/schemas/usage-beam';
	import type { TopologyState } from '$lib/schemas/usage-topology-view';

	type Point = { x: number; y: number };

	type Props = {
		/** Where the message comes from, which is the line's start. */
		from: Point;
		/** Where it goes, which is the line's end and where its node is drawn. */
		to: Point;
		/** Which hop of the path this is, stated on the element so a test can name it. */
		edge: string;
		/** This hop's own identity, which is what keys the turbulence filter: an active node carries a beam on two hops at once. */
		id: string;
		state: TopologyState;
		/** Frames are arriving on the connection that is open now, so the beam may move. */
		live: boolean;
	};

	let { from, to, edge, id, state, live }: Props = $props();

	const routing = $derived(state === 'active' && live);
	const filter = $derived(`url(#beam-${id})`);

	const orbs = beamOrbs();
	const sparks = beamSparks();

	/** The geometry every stroke of one hop shares. */
	const geometry = $derived({
		x1: from.x,
		y1: from.y,
		x2: to.x,
		y2: to.y,
		pathLength: 100,
		'vector-effect': 'non-scaling-stroke'
	});

	/** The one line a hop that is not carrying a beam is drawn as. */
	const EDGE: Record<TopologyState, string> = {
		active: 'stroke-[var(--color-ok)] [stroke-width:2]',
		last: 'stroke-[var(--color-warn)] [stroke-width:1.5]',
		error: 'stroke-[var(--color-danger)] [stroke-width:2]',
		idle: 'stroke-[var(--color-border)] [stroke-width:1]'
	};

	/** The three strokes of a routing hop, widest and faintest first. */
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

{#if routing}
	<!-- The reference's turbulence (`ProviderTopology.js:167-172`: baseFrequency 0.9, two octaves, seed
	     2), held still. It animates that frequency there with SMIL, which cannot honour a reduced-motion
	     preference, so the wobble is a texture here and the motion comes from the dashes. The displacement
	     is 0.4 rather than the reference's 3.5 because this path lives in a 100-unit box instead of pixel
	     coordinates. -->
	<defs>
		<filter id={`beam-${id}`} filterUnits="userSpaceOnUse" x="0" y="0" width="100" height="100">
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
	<line {...geometry} data-edge={edge} data-beam="halo" class={HALO} {filter} />
	<line
		{...geometry}
		data-edge={edge}
		data-beam="plasma"
		class={PLASMA}
		style={`stroke-dasharray:${BEAM_PLASMA_DASH}`}
	/>
	<line
		{...geometry}
		data-edge={edge}
		data-beam="core"
		class={CORE}
		style={`stroke-dasharray:${BEAM_CORE_DASH}`}
	/>
	{#each orbs as orb (orb.key)}
		<line
			{...geometry}
			data-edge={edge}
			data-beam="orb"
			class={`${DOT} ${TONE[orb.tone]} ${TRAVEL}`}
			style={`stroke-width:calc(${orb.width}px*var(--u)); stroke-dasharray:${BEAM_PARTICLE_DASH}; animation-duration:${orb.travel}s; animation-delay:${orb.delay}s`}
		/>
	{/each}
	{#each sparks as spark (spark.key)}
		<line
			{...geometry}
			data-edge={edge}
			data-beam="spark"
			class={`${DOT} ${TONE[spark.tone]} ${SPARK}`}
			style={`stroke-width:calc(${spark.width}px*var(--u)); stroke-dasharray:${BEAM_PARTICLE_DASH}; animation-duration:${spark.travel}s, ${spark.blink}s; animation-delay:${spark.delay}s, 0s`}
		/>
	{/each}
{:else}
	<line {...geometry} data-edge={edge} class={EDGE[state]} />
{/if}

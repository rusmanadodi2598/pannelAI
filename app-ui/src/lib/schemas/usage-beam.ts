// The live drawing's routing beam (docs/DRAFT/015-USAGE-NODE-MOTION-PARITY.md F1).
//
// Split from `usage-topology-view.ts` when the beam's own numbers arrived: that file answers where a node
// goes and which providers get one, this one answers what travels along the edge of an active one. Both are
// pure, so both are testable without a DOM, a clock, or a socket.
//
// Every value here is arithmetic over the path's normalized length rather than the edge's real length,
// which is why the drawing component renders what these return and computes nothing itself.
//
// The beam is the reference fork's (`ProviderTopology.js:137-245`, `globals.css:504-535` on `origin/master`,
// commit `a8c9d380`): a halo, a plasma and a core stroke, six orbs and five sparks. Its counts, widths,
// speeds and phases are the reference's own. The three dash periods are not, and each is named where it is
// declared: they are the panel's numbers, chosen so that every period divides the normalized path and a
// cycle has no seam.

/**
 * The two dashed strokes' patterns. The reference's are `8 6` and `14 10` along its own pixel paths, and
 * one animation cycle there moves the offset by -36 (`globals.css:504-506`), which is not a whole number of
 * either pattern: its dashes jump at the end of every cycle.
 *
 * The panel's paths are normalized to `pathLength="100"`, so the two patterns here are chosen with periods
 * that divide 100 exactly (10 and 25) and a cycle that moves the full -100. The loop is then seamless while
 * the layering, the widths and the speeds stay the reference's.
 */
export const BEAM_CORE_DASH = '6 4';
export const BEAM_PLASMA_DASH = '14 11';

/**
 * The two tones a beam dot takes: the status colour, and the colour the edge's core is drawn in.
 *
 * The reference's six orbs are yellow, cyan and white; the panel has one status colour, so the third tone
 * is the text colour. It is what makes a dot visible at all: measured in the browser, a dot in the status
 * colour alone merged into the plasma stroke and the beam read as one solid bar in both themes (draft 015
 * F2).
 */
export type BeamTone = 'ok' | 'text';

export type BeamParticle = {
	/** Stable key for the `{#each}`, named after what the dot is. */
	key: string;
	/** The dot's diameter in screen pixels, from the reference's own circle radii. */
	width: number;
	/** Seconds for one dot to cross the whole edge. */
	travel: number;
	/** Seconds of offset that keeps the dots out of step, applied as a negative delay. */
	delay: number;
	tone: BeamTone;
};

export type BeamSpark = BeamParticle & {
	/** Seconds for one blink, which the reference gives a shorter cycle than the travel. */
	blink: number;
};

/**
 * One dot, and nothing else, on a path that is 100 units long.
 *
 * This is what makes a particle a single traveller rather than a row of them: the dash paints one dot and
 * the 99.99 gap leaves the rest of the path bare, so the line carries exactly one dot. The cycle moves the
 * offset by -100, which is one whole period, so the dot leaves at the far end exactly as the loop restarts
 * at the near one.
 *
 * The first attempt here used the count itself as the period (`100 / 6`), which painted six dots per orb
 * line: the edge carried 61 dots where the reference carries 11, and the beam read as a chain in the
 * browser rather than as particles (draft 015 F2).
 */
export const BEAM_PARTICLE_DASH = '0.01 99.99';

/** The reference's six energy orbs: alternating sizes, three tones, each at its own speed and phase. */
const ORB_COUNT = 6;
/** The reference's five sparks, which blink as they travel. */
const SPARK_COUNT = 5;

/**
 * A duration or a delay as the number a stylesheet would carry.
 *
 * The reference's formulas are decimal fractions (`0.4 + i * 0.08`), and binary floating point turns some
 * of them into `0.44999999999999996`. That number would land in the DOM verbatim and in a test's
 * expectation, so it is rounded here, once, at the point the arithmetic happens.
 */
function seconds(value: number): number {
	return Number(value.toFixed(3));
}

/**
 * The orbs, sized `i % 2` and toned `i % 3` exactly as the reference alternates them.
 *
 * The reference's third tone is white and its second is cyan; here the third tone is the text colour, which
 * is near-black in the light theme and near-white in the dark one. It is the only second tone the panel
 * has, and without it a dot is invisible against the plasma (see `BeamTone`).
 */
export function beamOrbs(): BeamParticle[] {
	return Array.from({ length: ORB_COUNT }, (_, index) => ({
		key: `orb-${index}`,
		width: index % 2 === 0 ? 8 : 5,
		travel: seconds(0.4 + index * 0.08),
		delay: seconds(-(index * 0.09)),
		tone: index % 3 === 2 ? 'text' : 'ok'
	}));
}

/** The sparks, which travel faster than the orbs and blink on a cycle of their own. */
export function beamSparks(): BeamSpark[] {
	return Array.from({ length: SPARK_COUNT }, (_, index) => ({
		key: `spark-${index}`,
		width: 3.6,
		travel: seconds(0.28 + index * 0.05),
		delay: seconds(-(index * 0.11)),
		blink: seconds(0.35 + (index % 3) * 0.1),
		tone: 'ok' as BeamTone
	}));
}

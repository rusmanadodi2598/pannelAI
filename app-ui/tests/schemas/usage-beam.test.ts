// Beam derivation tests (src/lib/schemas/usage-beam.ts).
//
// The beam's counts, widths and phases are the reference fork's: six orbs and five sparks per routing edge,
// and three strokes at two dash patterns. Its travel times are the panel's, slowed so a packet can be
// followed from source to target. They live in the schema rather than in the drawing so the component
// computes nothing, and the rows here are the ones a reader cannot check by looking at the screen: the
// counts, the formulas, and the one invariant the motion depends on, that every pattern divides the
// normalized path a whole number of times so a cycle has no seam. That last one is the panel's own number
// rather than the reference's, which is why it is measured rather than assumed.

import { describe, expect, it } from 'vitest';
import {
	BEAM_CORE_DASH,
	BEAM_PARTICLE_DASH,
	BEAM_PLASMA_DASH,
	beamOrbs,
	beamSparks
} from '$lib/schemas/usage-beam';

/** A dash pattern's period, which is one dash plus its gap. */
function period(pattern: string): number {
	const [dash, gap] = pattern.split(' ').map(Number);
	return dash + gap;
}

describe('beamOrbs', () => {
	it('draws the six orbs of the reference fork, keyed so the drawing can order them', () => {
		expect(beamOrbs().map((orb) => orb.key)).toEqual([
			'orb-0',
			'orb-1',
			'orb-2',
			'orb-3',
			'orb-4',
			'orb-5'
		]);
	});

	it('alternates the two sizes the reference alternates, in screen pixels', () => {
		expect(beamOrbs().map((orb) => orb.width)).toEqual([8, 5, 8, 5, 8, 5]);
	});

	it('gives every orb its own speed and phase, so they do not travel as one row', () => {
		const orbs = beamOrbs();

		expect(orbs.map((orb) => orb.travel)).toEqual([0.9, 0.96, 1.02, 1.08, 1.14, 1.2]);
		expect(orbs.map((orb) => orb.delay)).toEqual([0, -0.09, -0.18, -0.27, -0.36, -0.45]);
	});

	it('tones every third orb away from the status colour, so a dot is visible against the beam', () => {
		// The reference's yellow, cyan and white become the panel's one status colour plus its text colour;
		// a dot in the status colour alone merged into the plasma stroke in the browser.
		expect(beamOrbs().map((orb) => orb.tone)).toEqual(['ok', 'ok', 'text', 'ok', 'ok', 'text']);
	});
});

describe('beamSparks', () => {
	it('draws the five sparks of the reference fork', () => {
		expect(beamSparks().map((spark) => spark.key)).toEqual([
			'spark-0',
			'spark-1',
			'spark-2',
			'spark-3',
			'spark-4'
		]);
	});

	it('sends them across faster than the orbs, each at its own speed', () => {
		expect(beamSparks().map((spark) => spark.travel)).toEqual([0.62, 0.67, 0.72, 0.77, 0.82]);
		expect(beamSparks().map((spark) => spark.delay)).toEqual([0, -0.11, -0.22, -0.33, -0.44]);
	});

	it("carries one calm phase, not the reference's hard blink", () => {
		expect(beamSparks().map((spark) => spark.blink)).toEqual([1.1, 1.1, 1.1, 1.1, 1.1]);
	});
});

describe('the beam dash patterns', () => {
	it('divides the normalized path a whole number of times, which is what keeps a cycle seamless', () => {
		const patterns = [BEAM_CORE_DASH, BEAM_PLASMA_DASH, BEAM_PARTICLE_DASH];

		for (const pattern of patterns) {
			const cycles = 100 / period(pattern);
			expect(Math.abs(cycles - Math.round(cycles))).toBeLessThan(0.001);
		}
	});

	it('paints exactly one dot per particle line, with the rest of the path left bare', () => {
		// The row that the browser pass earned: with the count as the period, each orb line painted six
		// dots and the edge carried a chain of them instead of six travellers.
		expect(BEAM_PARTICLE_DASH.startsWith('0.01 ')).toBe(true);
		expect(period(BEAM_PARTICLE_DASH)).toBe(100);

		const gap = Number(BEAM_PARTICLE_DASH.split(' ')[1]);
		expect(gap).toBeGreaterThan(90);
	});
});

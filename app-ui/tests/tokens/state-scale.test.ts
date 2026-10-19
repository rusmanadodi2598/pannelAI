// The interaction-state scale is measured rather than eyeballed (DESIGN.md §3.5).
//
// A state step is sized in L*, not by contrast ratio: a wash and its ground are deliberately close, so the
// pair reads about 1.08:1 whatever the step is, while the eye takes 3 L* as a flicker and 7 L* as a state.
// The floors sit under the smallest step measured (4.8 L*, the accent fill) so a re-tune has room and the
// 3.1 that failed cannot come back. Label rows assert only the pairings a control renders: a muted glyph
// lifts to full text at hover, so it never sits on a pressed wash.

import { describe, expect, it } from 'vitest';
import {
	composite,
	contrast,
	isOpaque,
	lab,
	resolveToken,
	THEMES,
	type Tokens
} from '../support/contrast';

const MIN_STEP = 4;
const MIN_RAMP = 8;

type Ink = { token: string; steps: readonly ('hover' | 'press' | 'rest')[] };
type Ramp = {
	label: string;
	steps: readonly [string, string, string];
	ground: string | null;
	inks: Ink[];
};

// The ramps a control can be drawn from: the neutral wash on each of the two grounds it lands on, the
// accent fill, and the destructive ramp in both of its shapes. A destructive control is either a commit
// in a dialog (the danger colour as its fill, with `danger-ink` on top) or one quiet glyph among four in
// a table row (the danger colour as its label, stepping along the same ramp).
const RAMPS: Ramp[] = [
	{
		label: 'the neutral wash on the page surface',
		steps: ['surface', 'state-hover', 'state-press'],
		ground: 'surface',
		inks: [
			{ token: 'text', steps: ['hover', 'press'] },
			{ token: 'text-muted', steps: ['hover'] }
		]
	},
	{
		label: 'the neutral wash on a raised panel',
		steps: ['surface-2', 'state-hover', 'state-press'],
		ground: 'surface-2',
		inks: [
			{ token: 'text', steps: ['hover', 'press'] },
			{ token: 'text-muted', steps: ['hover'] }
		]
	},
	{
		label: 'the accent fill',
		steps: ['accent', 'accent-hover', 'accent-press'],
		ground: null,
		inks: [{ token: 'accent-text', steps: ['hover', 'press'] }]
	},
	{
		label: 'the destructive fill',
		steps: ['danger', 'danger-hover', 'danger-press'],
		ground: null,
		inks: [{ token: 'danger-ink', steps: ['hover', 'press'] }]
	},
	...(['the page surface', 'a raised panel'] as const).map((groundLabel, index): Ramp => {
		const ground = index === 0 ? 'surface' : 'surface-2';

		return {
			label: `the destructive glyph on ${groundLabel}`,
			steps: [ground, 'state-hover', 'state-press'],
			ground,
			inks: [
				{ token: 'danger', steps: ['rest'] },
				{ token: 'danger-hover', steps: ['hover'] },
				{ token: 'danger-press', steps: ['press'] }
			]
		};
	})
];

function token(tokens: Tokens, name: string): string {
	const value = resolveToken(tokens, name);
	expect(value, `${name} is not declared`).toBeDefined();
	return value as string;
}

// The colour a reader receives: a translucent step is composited onto the ground it is painted over,
// because that is the layer under it in the shell.
function painted(tokens: Tokens, name: string, ramp: Ramp): string {
	const value = token(tokens, name);
	if (ramp.ground === null || isOpaque(value)) return value;

	const ground = token(tokens, ramp.ground);
	return composite(value, ground) ?? ground;
}

const lightnessOf = (tokens: Tokens, name: string, ramp: Ramp): number =>
	lab(painted(tokens, name, ramp))[0];

describe('interaction state scale', () => {
	for (const [theme, tokens] of THEMES) {
		describe(`${theme} theme`, () => {
			for (const ramp of RAMPS) {
				it(`steps ${ramp.label} by at least ${MIN_STEP} L* each and ${MIN_RAMP} across`, () => {
					const [rest, hover, press] = ramp.steps.map((name) => lightnessOf(tokens, name, ramp));

					expect(Math.abs(hover - rest), `${ramp.label} hover step`).toBeGreaterThanOrEqual(
						MIN_STEP
					);
					expect(Math.abs(press - hover), `${ramp.label} press step`).toBeGreaterThanOrEqual(
						MIN_STEP
					);
					expect(Math.abs(press - rest), `${ramp.label} whole ramp`).toBeGreaterThanOrEqual(
						MIN_RAMP
					);
				});

				it(`keeps every label on ${ramp.label} at AA`, () => {
					for (const ink of ramp.inks) {
						for (const step of ink.steps) {
							const name = ramp.steps[step === 'rest' ? 0 : step === 'hover' ? 1 : 2];
							const ratio = contrast(
								token(tokens, ink.token),
								painted(tokens, name, ramp),
								token(tokens, 'surface')
							);

							expect(ratio, `${ink.token} on ${ramp.label} at ${step}`).toBeGreaterThanOrEqual(4.5);
						}
					}
				});
			}

			// The direction is the theme's own: DESIGN.md §3.2 gives `surface-3` the pressed row, so
			// further in is darker in light and lighter in dark. A ramp that moved the other way would
			// read as a raised control while being pressed.
			it('steps the same way its surface ramp steps', () => {
				const neutral = lightnessOf(tokens, 'surface-3', {
					label: 'rows',
					steps: ['surface-3', 'surface-3', 'surface-3'],
					ground: null,
					inks: []
				});
				const sign = Math.sign(neutral - lightnessOf(tokens, 'surface', RAMPS[0]));

				for (const ramp of RAMPS) {
					const press = lightnessOf(tokens, ramp.steps[2], ramp);
					const rest = lightnessOf(tokens, ramp.steps[0], ramp);

					expect(Math.sign(press - rest), `${ramp.label} direction`).toBe(sign);
				}
			});

			// The accent steps are the same coral at two depths, not two more accents (R-29). Held to
			// chroma and hue, because L* is the axis the ramp is allowed to move along. The band is
			// +/-15%: a coral brightened toward its ceiling necessarily gives up some chroma, which is
			// the gamut doing it rather than a second hue being introduced.
			it('keeps the accent steps inside the one accent', () => {
				const base = lab(token(tokens, 'accent'));
				const baseChroma = Math.hypot(base[1], base[2]);
				const baseHue = (Math.atan2(base[2], base[1]) * 180) / Math.PI;

				for (const name of ['accent-hover', 'accent-press']) {
					const step = lab(token(tokens, name));
					const drift = Math.hypot(step[1], step[2]) / baseChroma;

					expect(drift, `${name} chroma drift`).toBeGreaterThanOrEqual(0.85);
					expect(drift, `${name} chroma drift`).toBeLessThanOrEqual(1.15);
					expect(
						Math.abs((Math.atan2(step[2], step[1]) * 180) / Math.PI - baseHue),
						`${name} hue drift`
					).toBeLessThan(5);
				}
			});
		});
	}
});

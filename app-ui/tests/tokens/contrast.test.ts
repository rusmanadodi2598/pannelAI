// Contrast tests for the shipped token layer.
//
// The tokens are read from src/app.css rather than restated here, so this test measures what the panel
// actually renders. R-25 requires WCAG AA: 4.5:1 for normal text and 3:1 for large text. A pair that
// fails is a defect in the palette, not in the test, and the fix belongs in app.css.
//
// Design direction lives in DESIGN.md §3. The palette there is one accent plus warm neutrals, and §3.3
// records the measured ratio for every pair asserted below. The math and the token reader live in
// tests/support/contrast.ts so a second suite can measure the same tokens without restating the formula.

import { existsSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { contrast, resolveToken, stylesheet, THEMES, type Tokens } from '../support/contrast';

// Pairs the panel renders as text, so a regression breaks a real screen.
const TEXT_PAIRS: { fg: string; bg: string; label: string }[] = [
	{ fg: 'text', bg: 'surface', label: 'body text on the page surface' },
	{ fg: 'text', bg: 'surface-2', label: 'body text on a raised panel' },
	{ fg: 'text', bg: 'surface-3', label: 'body text on a selected row' },
	{ fg: 'text-muted', bg: 'surface', label: 'muted text on the page surface' },
	{ fg: 'text-muted', bg: 'surface-2', label: 'muted text on a raised panel' },
	{ fg: 'text-muted', bg: 'surface-3', label: 'muted text on a selected row' },
	{ fg: 'accent', bg: 'surface', label: 'accent text on the page surface' },
	{ fg: 'accent', bg: 'surface-2', label: 'accent text on a raised panel' },
	{ fg: 'accent-text', bg: 'accent', label: 'accent button label' },
	{ fg: 'ok', bg: 'surface-2', label: 'success status on a raised panel' },
	{ fg: 'warn', bg: 'surface-2', label: 'warning status on a raised panel' },
	{ fg: 'danger', bg: 'surface-2', label: 'danger status on a raised panel' },
	// The live drawing's own two text pairs: the routing node's label on the node's own surface, and the
	// in-flight count on the status fill it sits in (draft 013 F1).
	{ fg: 'ok', bg: 'surface', label: 'routing provider label on the live drawing' },
	{ fg: 'surface', bg: 'ok', label: 'in-flight count on the live drawing' },
	// The sidebar is its own surface and carries navigation labels at the AA floor.
	{ fg: 'sidebar-foreground', bg: 'sidebar', label: 'navigation label on the sidebar' },
	{ fg: 'sidebar-accent-foreground', bg: 'sidebar-accent', label: 'active navigation label' }
];

// Non-text pairs. A graphic needs 3:1. A border is a seam rather than a WCAG pair, so it is held to a
// stated minimum: visible as a division, never readable as text.
const GRAPHIC_PAIRS: { fg: string; bg: string; floor: number; label: string }[] = [
	{ fg: 'accent', bg: 'surface-3', floor: 3, label: 'accent marker on a selected row' },
	{ fg: 'accent', bg: 'sidebar-accent', floor: 3, label: 'accent marker on the active row' },
	{ fg: 'accent', bg: 'sidebar', floor: 3, label: 'accent marker against the sidebar' },
	// The changelog marks each release with a 3px bar on the row's own surface, and the bar is the only
	// place `warn` is a graphic rather than text.
	{ fg: 'warn', bg: 'surface', floor: 3, label: 'warning marker on a release row' },
	// The live drawing marks a provider with a dot and an edge on the page's own surface, which is where
	// `ok` and `danger` are graphics rather than text. `warn` is covered by the release row above.
	{ fg: 'ok', bg: 'surface', floor: 3, label: 'in-flight marker on the live drawing' },
	{ fg: 'danger', bg: 'surface', floor: 3, label: 'error marker on the live drawing' },
	{ fg: 'border', bg: 'surface', floor: 1.2, label: 'surface seam' },
	{ fg: 'border', bg: 'surface-2', floor: 1.2, label: 'raised panel seam' }
];

// Tokens the panel cannot render without. A token declared in one theme and missing in the other is the
// failure mode R-34 calls a broken mode, so both sides are required.
const REQUIRED = [
	'surface',
	'surface-2',
	'surface-3',
	'border',
	'text',
	'text-muted',
	'accent',
	'accent-text',
	'ok',
	'warn',
	'danger',
	'sidebar',
	'sidebar-foreground',
	'sidebar-accent',
	'sidebar-accent-foreground',
	'sidebar-border',
	'sidebar-ring'
];

function assertPair(tokens: Tokens, fg: string, bg: string, floor: number, label: string): void {
	const foreground = resolveToken(tokens, fg);
	const background = resolveToken(tokens, bg);

	expect(foreground, `${fg} is not declared`).toBeDefined();
	expect(background, `${bg} is not declared`).toBeDefined();

	const page = resolveToken(tokens, 'surface') as string;
	const ratio = contrast(foreground as string, background as string, page);

	expect(
		ratio,
		`${fg} (${foreground}) on ${bg} (${background}) is ${ratio.toFixed(2)}:1, below ${floor}:1 for ${label}`
	).toBeGreaterThanOrEqual(floor);
}

describe('token layer', () => {
	it('declares every token in both themes', () => {
		for (const token of REQUIRED) {
			expect(resolveToken(THEMES[0][1], token), `light theme is missing ${token}`).toBeDefined();
			expect(resolveToken(THEMES[1][1], token), `dark theme is missing ${token}`).toBeDefined();
		}
	});

	it('resolves every alias in both themes', () => {
		for (const [theme, tokens] of THEMES) {
			for (const name of Object.keys(tokens)) {
				expect(resolveToken(tokens, name), `${theme} ${name} does not resolve`).toBeDefined();
			}
		}
	});

	for (const [theme, tokens] of THEMES) {
		describe(`${theme} theme contrast`, () => {
			for (const pair of TEXT_PAIRS) {
				it(`meets AA for ${pair.label}`, () => {
					assertPair(tokens, pair.fg, pair.bg, 4.5, pair.label);
				});
			}

			for (const pair of GRAPHIC_PAIRS) {
				it(`meets ${pair.floor}:1 for ${pair.label}`, () => {
					assertPair(tokens, pair.fg, pair.bg, pair.floor, pair.label);
				});
			}
		});

		// Neutral surfaces do not count toward the cap (R-29); ok, warn, and danger are status colours,
		// not brand colours, so the brand palette is one accent plus the neutrals. Checked per theme so a
		// second accent added to only one theme cannot slip through. DESIGN.md §3.1 records why the light
		// accent is not the raw legacy coral.
		it(`keeps exactly one accent family in the ${theme} theme`, () => {
			const accentTokens = Object.keys(tokens)
				.filter((token) => token.startsWith('accent'))
				.sort();

			expect(accentTokens, `${theme} theme accent tokens`).toEqual(['accent', 'accent-text']);
		});
	}
});

describe('typography', () => {
	// DESIGN.md §4: Inter, self-hosted, because a font CDN is excluded by SPEC-UI §11.7.
	it('puts Inter first in the sans stack', () => {
		expect(stylesheet).toMatch(/--font-sans:\s*'Inter'/);
	});

	it('ships the Inter variable font locally', () => {
		expect(existsSync(resolve(process.cwd(), 'static/fonts/InterVariable.woff2'))).toBe(true);
	});

	it('declares a font-face for the shipped file', () => {
		expect(stylesheet).toMatch(/@font-face\s*\{[\s\S]*?InterVariable\.woff2/);
	});

	it('references no third-party font host', () => {
		expect(stylesheet).not.toMatch(/fonts\.(googleapis|gstatic)\.com/);
	});
});

describe('palette hygiene', () => {
	// The alias layer exists so a colour is decided once. A hex value in the alias block would be a second
	// palette to keep in sync, which is how a token layer drifts.
	it('keeps raw hex values out of the alias block', () => {
		const alias = stylesheet.match(/@theme inline\s*\{([\s\S]*?)\n\}/)?.[1] ?? '';
		expect(alias).not.toMatch(/#[0-9a-fA-F]{6}/);
	});

	// A gradient, glow, or blur is an accent technique (R-01, R-10, R-13), and DESIGN.md §5 states the
	// panel uses none. Asserted rather than assumed so one cannot appear without a decision.
	it('uses no gradient, glow, or backdrop blur', () => {
		expect(stylesheet).not.toMatch(
			/linear-gradient|radial-gradient|backdrop-filter|filter:\s*blur/
		);
	});
});

// DESIGN.md §6: the active navigation row is marked with the one accent. Its wash is a translucent
// rgb() mix, so the channel triple has to be the theme's accent rather than merely a tint of some
// colour. This is what caught the dark wash carrying the warn triple and rendering the active row
// yellow in dark mode and coral in light.
describe('sidebar active wash', () => {
	const HEX = /^#([0-9a-fA-F]{6})$/;
	const RGB = /^rgb\(\s*(\d+)\s+(\d+)\s+(\d+)\s*\/\s*[\d.]+\s*\)$/;

	// Normalizes a hex or `rgb(r g b / a)` token to a `"r g b"` triple. A value that is neither is
	// returned unchanged, so an unexpected format fails the comparison instead of passing silently.
	function triple(value: string | undefined): string {
		if (value === undefined) return 'missing';

		const hex = value.match(HEX);
		if (hex) {
			const channels = Number.parseInt(hex[1], 16);
			return `${(channels >> 16) & 0xff} ${(channels >> 8) & 0xff} ${channels & 0xff}`;
		}

		const rgb = value.match(RGB);
		if (rgb) return `${Number(rgb[1])} ${Number(rgb[2])} ${Number(rgb[3])}`;

		return value;
	}

	for (const [theme, tokens] of THEMES) {
		it(`derives the ${theme} active-row wash from the accent`, () => {
			expect(triple(resolveToken(tokens, 'sidebar-accent'))).toBe(
				triple(resolveToken(tokens, 'accent'))
			);
		});
	}
});

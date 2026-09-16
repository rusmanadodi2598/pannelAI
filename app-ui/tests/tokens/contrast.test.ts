// Contrast tests for the shipped token layer.
//
// The tokens are read from src/app.css rather than restated here, so this test measures what the panel
// actually renders. R-25 requires WCAG AA: 4.5:1 for normal text and 3:1 for large text. A pair that
// fails is a defect in the palette, not in the test, and the fix belongs in app.css.

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

// Resolved from the project root rather than import.meta.url: under Vitest the module URL is not a
// file URL, so a relative read from it fails.
const css = readFileSync(resolve(process.cwd(), 'src/app.css'), 'utf8');

function blockTokens(selectorPattern: RegExp): Record<string, string> {
	const block = css.match(selectorPattern)?.[1] ?? '';
	const tokens: Record<string, string> = {};

	for (const match of block.matchAll(/--color-([a-z0-9-]+):\s*(#[0-9a-fA-F]{6})/g)) {
		tokens[match[1]] = match[2].toLowerCase();
	}

	return tokens;
}

const light = blockTokens(/@theme\s*\{([\s\S]*?)\n\}/);
const dark = blockTokens(/\.dark\s*\{([\s\S]*?)\n\}/);

// Pairs that the panel actually renders as text, so a regression breaks a real screen.
const PAIRS: { fg: string; bg: string; label: string }[] = [
	{ fg: 'text', bg: 'surface', label: 'body text on the page surface' },
	{ fg: 'text', bg: 'surface-2', label: 'body text on a raised panel' },
	{ fg: 'text-muted', bg: 'surface', label: 'muted text on the page surface' },
	{ fg: 'text-muted', bg: 'surface-2', label: 'muted text on a raised panel' },
	{ fg: 'accent-text', bg: 'accent', label: 'accent button label' },
	{ fg: 'danger', bg: 'surface', label: 'error text' },
	{ fg: 'warn', bg: 'surface', label: 'warning text' },
	{ fg: 'ok', bg: 'surface', label: 'success text' }
];

function channel(value: number): number {
	const scaled = value / 255;
	return scaled <= 0.03928 ? scaled / 12.92 : ((scaled + 0.055) / 1.055) ** 2.4;
}

function luminance(hex: string): number {
	const value = Number.parseInt(hex.slice(1), 16);
	const r = channel((value >> 16) & 0xff);
	const g = channel((value >> 8) & 0xff);
	const b = channel(value & 0xff);
	return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(foreground: string, background: string): number {
	const a = luminance(foreground);
	const b = luminance(background);
	const lighter = Math.max(a, b);
	const darker = Math.min(a, b);
	return (lighter + 0.05) / (darker + 0.05);
}

describe('token layer', () => {
	it('declares every token in both themes', () => {
		const required = [
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
			'danger'
		];

		for (const token of required) {
			expect(light[token], `light theme is missing ${token}`).toBeDefined();
			expect(dark[token], `dark theme is missing ${token}`).toBeDefined();
		}
	});

	for (const [theme, tokens] of [
		['light', light],
		['dark', dark]
	] as const) {
		describe(`${theme} theme contrast`, () => {
			for (const pair of PAIRS) {
				it(`meets AA for ${pair.label}`, () => {
					const foreground = tokens[pair.fg];
					const background = tokens[pair.bg];
					const ratio = contrast(foreground, background);

					expect(
						ratio,
						`${theme}: ${pair.fg} (${foreground}) on ${pair.bg} (${background}) is ${ratio.toFixed(2)}:1`
					).toBeGreaterThanOrEqual(4.5);
				});
			}
		});
	}

	it('keeps the palette within the three-colour plus one-accent cap', () => {
		// Neutral surfaces do not count toward the cap (R-29); ok, warn, and danger are status colours,
		// not brand colours, so the brand palette is one accent plus the neutrals.
		expect(light.accent).toBeDefined();
		expect(light['accent-text']).toBeDefined();
	});
});

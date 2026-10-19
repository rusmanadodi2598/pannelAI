// WCAG contrast math and token reading for the panel's tests.
//
// Kept out of the test file so a second suite (the shell's own state, a future component test) can measure
// the same tokens without restating the formula. Everything here reads `src/app.css` rather than accepting
// a palette, so a test measures what the panel actually ships.

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';

const TOKEN = /--color-([a-z0-9-]+):\s*(#[0-9a-fA-F]{6}|var\(--color-[a-z0-9-]+\)|rgb\([^)]*\))/g;

const css = readFileSync(resolve(process.cwd(), 'src/app.css'), 'utf8');

export const stylesheet = css;

// The palette is app.css plus every local token file it imports, because the interaction-state scale
// lives in `src/lib/states.css` on the line budget the same way `src/lib/motion.css` does. A bare
// specifier (`tailwindcss`, `tw-animate-css`) is a dependency's own CSS rather than the panel's palette,
// so it is not followed; only a relative import is read.
const tokenSource = [
	css,
	...[...css.matchAll(/@import\s+'(\.\/[^']+)'/g)].map((match) =>
		readFileSync(resolve(process.cwd(), 'src', match[1].slice(2)), 'utf8')
	)
].join('\n');

// A selector can appear more than once (the dark block is extended by the sidebar tint), so every
// occurrence is merged rather than only the first.
function blockTokens(pattern: RegExp): Record<string, string> {
	const tokens: Record<string, string> = {};

	for (const block of tokenSource.matchAll(pattern)) {
		for (const match of block[1].matchAll(TOKEN)) {
			const value = match[2];
			tokens[match[1]] = value.startsWith('#') ? value.toLowerCase() : value.trim();
		}
	}

	return tokens;
}

// Both theme blocks are read: the literal one holds the palette, and the `inline` one holds the primitive
// layer's aliases. Merging them is what lets a `sidebar` pair be measured at all, because that token only
// exists as an alias.
export const lightTokens = blockTokens(/@theme(?:\s+inline)?\s*\{([\s\S]*?)\n\}/g);
export const darkTokens = {
	...lightTokens,
	...blockTokens(/\.dark\s*\{([\s\S]*?)\n\}/g)
};

export type Tokens = Record<string, string>;

export const THEMES: readonly (readonly [string, Tokens])[] = [
	['light', lightTokens],
	['dark', darkTokens]
];

// The alias layer is one level deep (`--color-sidebar-foreground` points at `--color-text`). Resolving it
// here keeps the token file free of duplicated hex values while still measuring the pair the browser
// renders. A cycle returns undefined, which reads as a missing token.
export function resolveToken(tokens: Tokens, name: string): string | undefined {
	const seen = new Set<string>();
	let current = name;

	while (!seen.has(current)) {
		seen.add(current);
		const value = tokens[current];
		if (value === undefined) return undefined;
		if (value.startsWith('#')) return value;
		if (value.startsWith('rgb(')) return value;

		const inner = value.match(/^var\(--color-([a-z0-9-]+)\)$/)?.[1];
		if (inner === undefined) return undefined;
		current = inner;
	}

	return undefined;
}

const RGB = /^rgb\(\s*(\d+)\s+(\d+)\s+(\d+)\s*\/\s*([\d.]+)\s*\)$/;

export function isOpaque(value: string | undefined): boolean {
	return typeof value === 'string' && value.startsWith('#');
}

// Composites a translucent token over an opaque one. The sidebar's active row is an accent wash, so the
// ratio has to be measured against what a reader's eye receives rather than against the raw channel triple
// with its alpha dropped.
export function composite(value: string | undefined, over: string | undefined): string | undefined {
	if (typeof value !== 'string' || typeof over !== 'string') return undefined;
	if (isOpaque(value)) return value;
	if (!isOpaque(over)) return undefined;

	const match = value.match(RGB);
	if (!match) return undefined;

	const alpha = Number(match[4]);
	const base = Number.parseInt(over.slice(1), 16);
	const channels = [1, 2, 3].map((index) => {
		const tint = Number(match[index]);
		const under = (base >> (8 * (3 - index))) & 0xff;
		return Math.round(tint * alpha + under * (1 - alpha));
	});

	return `#${channels.map((c) => c.toString(16).padStart(2, '0')).join('')}`;
}

function channel(value: number): number {
	const scaled = value / 255;
	return scaled <= 0.03928 ? scaled / 12.92 : ((scaled + 0.055) / 1.055) ** 2.4;
}

// The three linear-light channels of a hex colour, which is the form both the luminance sum and the
// XYZ matrix read. SRGB is assumed, as it is everywhere else in this file.
function toLinear(hex: string): [number, number, number] {
	const value = Number.parseInt(hex.slice(1), 16);
	return [channel((value >> 16) & 0xff), channel((value >> 8) & 0xff), channel(value & 0xff)];
}

export function luminance(hex: string): number {
	const [r, g, b] = toLinear(hex);
	return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

// `pageSurface` is the theme's opaque ground. Anything translucent in the pair is painted on top of it
// first, because that is the layer beneath a wash in the shell.
export function contrast(foreground: string, background: string, pageSurface: string): number {
	const ground = isOpaque(background)
		? background
		: (composite(background, pageSurface) ?? pageSurface);
	const ink = isOpaque(foreground) ? foreground : (composite(foreground, ground) ?? foreground);

	const a = luminance(ink);
	const b = luminance(ground);
	return (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);
}

// CIELAB, which is the measure a state step is sized by. A contrast ratio cannot express it: a hover
// wash and the ground under it are deliberately close, so the pair measures about 1.08:1 at any step
// size, while the eye reads 3 L* as a flicker and 7 L* as a state. L* is the step, a* and b* say which
// colour the step stayed in. A translucent token is measured as the colour it composites to, the same
// way a reader receives it.
export function lab(value: string, over?: string): [number, number, number] {
	const hex = isOpaque(value)
		? value
		: over === undefined
			? value
			: (composite(value, over) ?? value);
	const [r, g, b] = toLinear(hex);
	const f = (t: number): number => (t > 0.008856 ? Math.cbrt(t) : 7.787 * t + 16 / 116);
	const x = f((0.4124 * r + 0.3576 * g + 0.1805 * b) / 0.95047);
	const y = f(luminance(hex));
	const z = f((0.0193 * r + 0.1192 * g + 0.9505 * b) / 1.08883);

	return [116 * y - 16, 500 * (x - y), 200 * (y - z)];
}

// `L*` alone: the lightness a state step moves through.
export function lightness(value: string, over?: string): number {
	return lab(value, over)[0];
}

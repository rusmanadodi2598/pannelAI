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

// A selector can appear more than once (the dark block is extended by the sidebar tint), so every
// occurrence is merged rather than only the first.
function blockTokens(pattern: RegExp): Record<string, string> {
	const tokens: Record<string, string> = {};

	for (const block of css.matchAll(pattern)) {
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

export function luminance(hex: string): number {
	const value = Number.parseInt(hex.slice(1), 16);
	return (
		0.2126 * channel((value >> 16) & 0xff) +
		0.7152 * channel((value >> 8) & 0xff) +
		0.0722 * channel(value & 0xff)
	);
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

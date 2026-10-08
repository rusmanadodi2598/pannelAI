// The node box that the drawing's sizing unit is derived from, checked against the drawing's own markup.
//
// `NODE_MAX_WIDTH` is not a free number: it is the provider node at its largest, and every metric that makes
// it up is written as a Tailwind class inside `UsageTopologyDrawing.svelte`. jsdom runs no Tailwind cascade,
// so the class list is the only part of that rule a unit test can hold. This is the same limit
// `tests/tokens/contrast.test.ts` answers by reading the stylesheet instead of restating the tokens.
//
// It is worth a test because the two halves live in different files and nothing else ties them. The drawing
// computes `--u` as its own width over the width at which a node reaches this cap, so a metric that moves
// without the cap moving lets the drawing keep scaling past the point where a node is as wide as the panel
// ever draws one, which is the collision the cap exists to stop.

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { NODE_MAX_WIDTH } from '$lib/schemas/usage-topology-geometry';

const DRAWING = resolve(process.cwd(), 'src/lib/components/UsageTopologyDrawing.svelte');

// Tailwind's `border` utility is 1px on each edge, so a node box carries 2px across its width.
const BORDER_ACROSS = 2;

/** The provider node's own markup: the block the `{#each layout.nodes}` loop renders. */
function nodeBlock(): string {
	const source = readFileSync(DRAWING, 'utf-8');
	const start = source.indexOf('{#each layout.nodes');
	if (start === -1)
		throw new Error('the node loop has left the drawing; this pin needs its new home');
	return source.slice(start, source.indexOf('{/each}', start));
}

/** The pixel value a scaled utility carries, read from the class list rather than restated here. */
function scaledPixels(block: string, utility: string): number {
	const match = new RegExp(`${utility}-\\[calc\\((\\d+)px\\*var\\(--u\\)\\)\\]`).exec(block);
	if (!match) throw new Error(`${utility} is no longer scaled by --u on the node box`);
	return Number(match[1]);
}

/** One Tailwind class per entry. A class list is what the derivation is measured against, not a substring. */
function classTokens(block: string): string[] {
	return block.split(/[\s"'`]+/).filter(Boolean);
}

describe('the node box NODE_MAX_WIDTH is derived from', () => {
	it('sums its own scaled metrics to the cap the unit is measured against', () => {
		const block = nodeBlock();
		const across =
			scaledPixels(block, 'px') * 2 +
			scaledPixels(block, 'size') +
			scaledPixels(block, 'gap') +
			scaledPixels(block, 'max-w') +
			BORDER_ACROSS;

		expect(across).toBe(NODE_MAX_WIDTH);
	});

	// `-` is not a word character, so /\bborder\b/ also matches border-b and
	// border-[var(--color-border)]; the derivation counts only the bare utility's 1px.
	it('keeps the bare border utility the derivation counts', () => {
		expect(classTokens(nodeBlock())).toContain('border');
	});
});

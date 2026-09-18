// Sidebar responsive contract.
//
// DESIGN.md §8 gives three bands: a drawer under 768px, an icon rail from 768px to 1023px that expands
// over the content, and a full sidebar from 1024px. The drawer and the rail come from the primitive
// layer; the tablet overlay is the panel's own rule in src/app.css, so it is asserted here rather than
// left to a screenshot that nobody re-takes. R-12 and DESIGN.md §5 limit elevation to a dialog and the
// mobile drawer, which is why the overlay must not gain a shadow while it floats.

import { describe, expect, it } from 'vitest';
import { stylesheet } from '../support/contrast';

// Captures the body of the tablet media query. The lazy match stops at the first closing brace in the
// first column, which is the media query's own close rather than the nested rule's.
const TABLET_BLOCK =
	/@media\s*\(min-width:\s*768px\)\s*and\s*\(max-width:\s*1023px\)\s*\{([\s\S]*?)\n\}/;

describe('tablet sidebar behaviour', () => {
	it('declares a rule for the 768px to 1023px band', () => {
		expect(stylesheet, 'no tablet media query in src/app.css').toMatch(TABLET_BLOCK);
	});

	it('collapses the gap so an expanded sidebar floats over the content', () => {
		const block = stylesheet.match(TABLET_BLOCK)?.[1] ?? '';

		expect(block, 'the overlay does not target the sidebar gap').toContain('sidebar-gap');
		expect(block, 'the gap is not collapsed, so the content would reflow').toMatch(/width:\s*0\b/);
	});

	it('adds no elevation while it floats (R-12, DESIGN.md §5)', () => {
		const block = stylesheet.match(TABLET_BLOCK)?.[1] ?? '';

		expect(block).not.toMatch(/box-shadow|drop-shadow|filter:/);
	});
});

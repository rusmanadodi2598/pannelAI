// Every control the Skills screen renders carries a pressed colour (DESIGN.md §3.5).
//
// The screen's job is to hand over one line, so its controls are the screen: the copy control on the entry
// block and on each capability row, the off-site link beside them, the check that re-reads the sources,
// and the retry a failed catalog read leaves behind. The link was bare underline text, which is a control
// on the one device that has no hover to answer with.

import { cleanup, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import SkillsPage from '../../src/routes/skills/+page.svelte';
import { skillCatalog } from '../support/skill-catalog';
import { rowFor, stubSkills } from '../support/skills-stub';
import { expectControlStates, expectEveryControlHasStates } from '../support/control-states';

function controls(): HTMLElement[] {
	return Array.from(document.querySelectorAll('button, a[data-slot="button"]'));
}

async function loaded(): Promise<void> {
	await screen.findByText('3 of 3 sources are published.');
}

describe('Skills control states', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('gives every control on the screen a pressed colour', async () => {
		stubSkills(skillCatalog());
		render(SkillsPage);
		await loaded();

		// Three copy controls, three off-site links, and the check beside the count.
		const seen = controls();
		expect(seen.length).toBeGreaterThanOrEqual(7);
		expectEveryControlHasStates(seen, 'the Skills screen');
	});

	it('keeps the source link a link, and gives it the states anyway', async () => {
		stubSkills(skillCatalog());
		render(SkillsPage);
		await loaded();

		const link = within(rowFor('pannelai-chat')).getByRole('link', {
			name: 'Read it on GitHub: Chat'
		}) as HTMLAnchorElement;

		expect(link.getAttribute('href')).toMatch(/^https:\/\/github\.com\//);
		expect(link.getAttribute('target')).toBe('_blank');
		expect(link.getAttribute('rel')).toBe('noreferrer');
		expectControlStates(link);
	});

	it('gives the copy control the row the screen is for', async () => {
		// The install line is what an operator comes to take, and taking it is a press rather than a
		// glance, so the dense table shape does not apply here: the icon-only control keeps a 44px square.
		stubSkills(skillCatalog());
		render(SkillsPage);
		await loaded();

		const copies = screen.getAllByRole('button', { name: /Copy install line/ });
		expect(copies).toHaveLength(3);
		for (const copy of copies) expect(copy.className).toContain('size-11');
	});

	it('gives a failed catalog read its retry', async () => {
		stubSkills(
			{ error: { code: 'INTERNAL_ERROR', message: 'The catalog is unreadable.' } },
			() => 200,
			500
		);
		render(SkillsPage);

		expectControlStates(await screen.findByRole('button', { name: 'Try again' }));
	});

	it('leaves no control on a row whose source did not answer', async () => {
		// §6.10's honesty rule, re-asserted against the migrated markup: the states are only worth having
		// if a broken row still ships nothing pressable.
		stubSkills(skillCatalog(), (url) => (url.includes('/pannelai-image/') ? 404 : 200));
		render(SkillsPage);

		await screen.findByText('2 of 3 sources are published.');
		const image = rowFor('pannelai-image');
		expect(image.querySelectorAll('button, a[data-slot="button"]')).toHaveLength(0);
	});
});

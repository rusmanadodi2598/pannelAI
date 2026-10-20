// Every control on the Provider registry list carries the states (DESIGN.md §3.5).
//
// The screen rendered sixteen controls from hand-written class lists. The search and the two add actions
// painted a border and nothing else, so they were indistinguishable from the static panel behind them,
// and the six category chips changed only their text colour, which leaves a chosen filter and a hovered
// one looking the same. The chips are why the scale gained a selected step: a chosen control sits on the
// ground DESIGN.md §3.2 calls pressed, and its hover has to reach past that.
//
// The walk counts anchors too, because three of this screen's controls are links: an action link is a
// control (§3.5), and the registry row's two actions and the node row's name were bare underlines with no
// state rule of their own.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProvidersPage from '../../src/routes/providers/+page.svelte';
import { expectControlStates, expectEveryControlHasStates } from '../support/control-states';
import { provider, stubProviders } from '../support/providers-route-stub';
import { nodeRow } from '../support/provider-node-stub';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

async function controls(): Promise<HTMLElement[]> {
	stubProviders({ rows: [provider()] });
	render(ProvidersPage);
	await screen.findByRole('table');

	return Array.from(document.querySelectorAll('button, a[data-slot="button"]'));
}

describe('Provider registry control states', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('gives every control a hover and a pressed step, and no raw colour', async () => {
		// Search, refresh, six chips, the two add actions, the two pagination steps, and the row's Open link.
		const buttons = await controls();
		expect(buttons.length).toBeGreaterThanOrEqual(12);

		expectEveryControlHasStates(buttons, 'Provider registry');
	});

	it('keeps the three links links, and gives them the states anyway', async () => {
		stubProviders({ rows: [provider({ endpoint_count: 0 })], nodes: [nodeRow()] });
		render(ProvidersPage);
		await screen.findByRole('table');

		// The registry row offers both of its actions when it holds no endpoint, and the node row's name
		// routes to the screen that owns the node's facts.
		const cases: [string, string][] = [
			['Open', '/providers/openai'],
			['Add endpoint', '/providers/openai'],
			['Corp gateway', '/providers/openai-compatible-01J']
		];

		for (const [name, href] of cases) {
			const link = (await screen.findByRole('link', { name })) as HTMLAnchorElement;
			expect(link.getAttribute('href'), `${name} link`).toBe(href);
			expect(link.className, `${name} link lost its 44px target`).toContain('min-h-11');
			expectControlStates(link);
		}
	});

	it('reads the chosen filter from aria-pressed rather than from a class the screen assembles', async () => {
		const buttons = await controls();
		const chips = buttons.filter((button) => button.hasAttribute('aria-pressed'));

		expect(chips.length).toBe(6);
		for (const chip of chips) {
			expect(chip.className, 'the chosen chip steps off its own ground').toContain(
				'aria-[pressed=true]:hover:bg-[var(--color-state-deep)]'
			);
			expect(chip.className, 'the chosen chip keeps its border as the second cue').toContain(
				'aria-[pressed=true]:border-[var(--color-text)]'
			);
		}

		const media = screen.getByRole('button', { name: 'media' });
		expect(media.getAttribute('aria-pressed')).toBe('false');
		await fireEvent.click(media);
		expect(media.getAttribute('aria-pressed')).toBe('true');
	});
});

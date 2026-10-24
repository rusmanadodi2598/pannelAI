// Every control the console screen renders carries a pressed colour, not only a hover one (DESIGN.md §3.5).
//
// The screen asked the same question four times with a class written at the call site: the pause, the
// clear, the cross-link to `/logs`, and the retry in the failed state. The link carried no state rule at
// all, so on a touch layout it answered nothing whatsoever.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ConsoleLogPage from '../../src/routes/console-log/+page.svelte';
import { stubConsole } from '../support/console-stub';
import { expectControlStates, expectEveryControlHasStates } from '../support/control-states';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

function controls(): HTMLElement[] {
	return Array.from(document.querySelectorAll('button, a[data-slot="button"]'));
}

describe('Console log control states', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('gives every control on the screen a pressed colour', async () => {
		stubConsole();
		render(ConsoleLogPage);
		await screen.findByText(/gateway booted/);

		// The pause, the refresh, the clear, the cross-link, and the three the dialog carries: its close,
		// its dismiss, and its commit.
		const seen = controls();
		expect(seen.length).toBeGreaterThanOrEqual(4);
		expectEveryControlHasStates(seen, 'the console screen');
	});

	it('keeps the cross-link a link, and gives it the states anyway', async () => {
		stubConsole();
		render(ConsoleLogPage);
		await screen.findByText(/gateway booted/);

		const link = screen.getByRole('link', { name: 'Request logs' });
		expect(link.getAttribute('href')).toBe('/logs');
		expectControlStates(link);
	});

	it('steps the clear along the danger ramp rather than washing it in ink', async () => {
		// The commit that deletes waits in the dialog, so the trigger keeps its colour and its border and
		// takes no fill: an ink wash under a danger label measures 4.17:1 on a raised panel.
		stubConsole();
		render(ConsoleLogPage);
		await screen.findByText(/gateway booted/);

		const trigger = screen.getByRole('button', { name: 'Clear console buffer' });
		expect(trigger.className).toContain('text-[var(--color-danger)]');
		expect(trigger.className).toContain('hover:text-[var(--color-danger-hover)]');
		expect(trigger.className).toContain('active:text-[var(--color-danger-press)]');
		expect(trigger.className).toContain('border-[var(--color-danger)]');
	});

	it('gives the dialog its commit the filled destructive shape', async () => {
		stubConsole();
		render(ConsoleLogPage);
		await screen.findByText(/gateway booted/);

		await fireEvent.click(screen.getByRole('button', { name: 'Clear console buffer' }));

		const commit = await screen.findByRole('button', { name: 'Clear buffer' });
		expectControlStates(commit);
		expect(commit.className).toContain('bg-[var(--color-danger)]');
		expect(commit.className).toContain('text-[var(--color-danger-ink)]');
	});

	it('gives a failed read its retry', async () => {
		stubConsole({ status: 500 });
		render(ConsoleLogPage);

		expectControlStates(await screen.findByRole('button', { name: 'Try again' }));
	});

	it('makes the follow label a row a thumb can land on', async () => {
		// The label is the target, not the 16px box inside it: clicking the words toggles the checkbox, so
		// SPEC-UI §8.7's 44px applies to the label rather than to the input.
		stubConsole();
		render(ConsoleLogPage);
		await screen.findByText(/gateway booted/);

		expect(screen.getByText('Follow newest').closest('label')?.className).toContain('min-h-11');
	});
});

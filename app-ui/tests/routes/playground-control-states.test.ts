// Every control the Playground screen renders carries a pressed colour (DESIGN.md §3.5).
//
// Three controls were written at the call site and none of them painted under a press: the retry a failed
// model read leaves behind, and the composer's send and stop, which share one slot. The composer is the
// screen's only way to spend money, so the accent belongs to it, and the two rows that count the accent
// guard §3.4 rather than this file's own colour choice.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import PlaygroundPage from '../../src/routes/playground/+page.svelte';
import { expectControlStates, expectEveryControlHasStates } from '../support/control-states';
import { stallingAnswer, stubPlayground } from '../support/playground-page';

const MODELS = { data: [{ id: 'deepseek/chat', owned_by: 'deepseek' }] };
const READ_FROM = 'Read from GET /playground/models.';

const ACCENT_FILL = 'bg-[var(--color-accent)]';

function controls(): HTMLElement[] {
	return Array.from(document.querySelectorAll('button'));
}

function filled(): HTMLElement[] {
	return controls().filter((button) => button.className.includes(ACCENT_FILL));
}

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('Playground control states', () => {
	it('gives the composer send its states, its 44px row, and the screen one accent', async () => {
		stubPlayground({ models: MODELS });
		render(PlaygroundPage);

		await screen.findByText(READ_FROM);
		expectEveryControlHasStates(controls(), 'the Playground screen');

		const send = screen.getByRole('button', { name: 'Send' });
		expectControlStates(send);
		expect(send.className).toContain('min-h-11');
		expect(send.getAttribute('type')).toBe('submit');

		// The send is the one primary action this screen has (§3.4), and it is the only thing wearing it.
		expect(send.className).toContain(ACCENT_FILL);
		expect(filled()).toHaveLength(1);
	});

	it('gives the stop the same slot, its states, and the accent, while a stream runs', async () => {
		stubPlayground({ models: MODELS, chat: stallingAnswer('partial') });
		render(PlaygroundPage);

		await screen.findByText(READ_FROM);
		await fireEvent.input(screen.getByLabelText('Message'), { target: { value: 'hi' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Send' }));

		const stop = await screen.findByRole('button', { name: 'Stop' });
		expectControlStates(stop);
		expect(stop.className).toContain(ACCENT_FILL);
		expect(screen.queryByRole('button', { name: 'Send' })).toBeNull();

		// One slot, one accent: the control that started the send is the control that stops it.
		expect(filled()).toHaveLength(1);
	});

	it('gives a failed model read its retry, on the ramp and still an underlined link-style action', async () => {
		stubPlayground({
			models: { error: { code: 'GATEWAY_ERROR', message: 'The gateway answered HTTP 502.' } },
			modelsStatus: 502
		});
		render(PlaygroundPage);

		const retry = await screen.findByRole('button', { name: 'Try again' });
		expectControlStates(retry);
		expect(retry.className).toContain('underline');
		expect(retry.className).not.toContain(ACCENT_FILL);
	});
});

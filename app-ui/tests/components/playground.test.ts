// Playground render tests: the states before a send (docs/SPEC-UI/001-SPEC-UI.md §6.15).
//
// The screen's first job is to say which of four things is true before it offers a control: the models are
// still being read, the panel has no key, the read failed, or the key routes no model. Only one of those
// four renders a composer, so the assertions here are mostly about what is absent. The last rows are the
// composer itself: the cost sentence is on screen before a send (§6.15 rule 4), and a message of
// whitespace is refused by the panel's own schema rather than by a disable attribute.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import PlaygroundPage from '../../src/routes/playground/+page.svelte';
import { stubPlayground } from '../support/playground-page';

const MODELS = {
	data: [{ id: 'deepseek/chat', owned_by: 'deepseek' }, { id: 'combo/fast' }]
};

const KEY_MISSING = {
	error: {
		code: 'PLAYGROUND_KEY_MISSING',
		message:
			'The panel has no gateway key configured. Set PANEL_PLAYGROUND_KEY in the panel server environment.'
	}
};

const READ_FROM = 'Read from GET /playground/models.';

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('PlaygroundPage model states', () => {
	it('says what it is reading while the model read is in flight', () => {
		vi.stubGlobal('fetch', () => new Promise(() => {}));
		render(PlaygroundPage);

		expect(screen.getByText('Reading the models this key can route')).toBeTruthy();
	});

	it('states the unconfigured panel key, naming the variable the route named', async () => {
		stubPlayground({ models: KEY_MISSING, modelsStatus: 503 });
		render(PlaygroundPage);

		expect(await screen.findByText('The panel has no gateway key')).toBeTruthy();
		// The name reaches the screen inside the panel's own sentence, which is the only place allowed to
		// carry it: the browser bundle must not name the variable (tests/server/playground-secret.test.ts).
		expect(screen.getByText(/PANEL_PLAYGROUND_KEY/)).toBeTruthy();
		expect(screen.getByText(/The browser is not asked for one/)).toBeTruthy();

		// No composer and no picker: a send control that cannot send is a dead control (R-26).
		expect(screen.queryByRole('button', { name: 'Send' })).toBeNull();
		expect(screen.queryByRole('combobox')).toBeNull();
	});

	it('reports a failed read and reads again when asked', async () => {
		const calls = stubPlayground({
			models: { error: { code: 'GATEWAY_ERROR', message: 'The gateway answered HTTP 502.' } },
			modelsStatus: 502
		});
		render(PlaygroundPage);

		expect(await screen.findByText('The model list could not be read')).toBeTruthy();
		expect(screen.getByText('The gateway answered HTTP 502.')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

		expect(calls.models).toBe(2);
	});

	it('states an empty model list instead of offering a picker with nothing in it', async () => {
		stubPlayground({ models: { data: [] } });
		render(PlaygroundPage);

		expect(await screen.findByText('This key routes no model')).toBeTruthy();
		expect(screen.queryByRole('combobox')).toBeNull();
		expect(screen.queryByRole('button', { name: 'Send' })).toBeNull();
	});
});

describe('PlaygroundPage composer', () => {
	it('offers the models the key routes, the first one selected, and the cost before any send', async () => {
		stubPlayground({ models: MODELS });
		render(PlaygroundPage);

		expect(await screen.findByText(READ_FROM)).toBeTruthy();
		expect(screen.getByText('2 models')).toBeTruthy();

		const select = screen.getByRole('combobox', { name: 'Model' }) as HTMLSelectElement;
		expect(select.value).toBe('deepseek/chat');
		expect(screen.getByRole('option', { name: 'deepseek/chat (deepseek)' })).toBeTruthy();
		expect(screen.getByRole('option', { name: 'combo/fast' })).toBeTruthy();

		// The cost, stated before the operator sends one rather than reported after it.
		expect(screen.getByText(/consumes the key quota and writes a usage record/)).toBeTruthy();
	});

	it('refuses a message of whitespace through the schema, and sends nothing', async () => {
		const calls = stubPlayground({ models: MODELS });
		render(PlaygroundPage);

		await screen.findByText(READ_FROM);
		await fireEvent.input(screen.getByLabelText('Message'), { target: { value: '   ' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Send' }));

		expect(await screen.findByText('Write a message to send.')).toBeTruthy();
		expect(calls.chatBodies).toEqual([]);
	});

	it('will not send an empty box', async () => {
		stubPlayground({ models: MODELS });
		render(PlaygroundPage);

		await screen.findByText(READ_FROM);
		const send = screen.getByRole('button', { name: 'Send' }) as HTMLButtonElement;

		expect(send.disabled).toBe(true);

		await fireEvent.input(screen.getByLabelText('Message'), { target: { value: 'hi' } });

		expect((screen.getByRole('button', { name: 'Send' }) as HTMLButtonElement).disabled).toBe(
			false
		);
	});
});

// Adding a custom provider (docs/SPEC-UI/001-SPEC-UI.md §6.3, docs/SPEC-API/001-SPEC-API.md §7.4).
//
// Two controls, one per compatible type, and the type is what decides the rest: an OpenAI-compatible node
// carries an api type and an Anthropic-compatible one refuses it, so the second button opens a dialog with
// no such field rather than one the gateway would refuse the body for. The API type is also what decides
// the endpoint, which is why the form shows the joined URL while the base URL is typed.
//
// The stub refuses what the gateway refuses, which is what makes the local-refusal case meaningful: a test
// that types a prefix with a slash and asserts no request was sent proves the panel caught it, not that
// the stub was lenient. The two refusals are checked apart because they come from different places: the
// draft's own rule sends nothing, while a taken prefix is the gateway's answer and must be rendered as it
// stated it, naming the provider that owns the prefix.

import { cleanup, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { stubProviderNodes } from '../support/provider-node-stub';
import { renderSection, select, type } from '../support/custom-provider-harness';
import { squashed, value } from '../support/dom';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('adding a custom provider', () => {
	it('opens the OpenAI dialog from its own button', async () => {
		renderSection(stubProviderNodes());

		await screen.getByRole('button', { name: 'Add OpenAI Compatible' }).click();

		expect(screen.getByRole('heading', { name: 'Add OpenAI Compatible' })).toBeTruthy();
		expect(screen.getByLabelText('API type')).toBeTruthy();
	});

	it('opens the Anthropic dialog without an API type field, which §7.4 refuses there', async () => {
		renderSection(stubProviderNodes());

		await screen.getByRole('button', { name: 'Add Anthropic Compatible' }).click();

		expect(screen.getByRole('heading', { name: 'Add Anthropic Compatible' })).toBeTruthy();
		expect(screen.queryByLabelText('API type')).toBeNull();
	});

	it('posts the typed node, then re-reads so the list shows what the gateway stored', async () => {
		const stub = stubProviderNodes();
		const rendered = renderSection(stub);

		await screen.getByRole('button', { name: 'Add OpenAI Compatible' }).click();
		await type('Name', 'Corp gateway');
		await type('Prefix', 'mycorp');
		await select('API type', 'responses');
		await type('Base URL', 'https://llm.example.com/v1/');
		await screen.getByRole('button', { name: 'Add the provider' }).click();

		await waitFor(() => expect(stub.creates).toHaveLength(1));
		expect(stub.creates[0]).toEqual({
			name: 'Corp gateway',
			prefix: 'mycorp',
			type: 'openai-compatible',
			api_type: 'responses',
			// The trailing slash is dropped by the draft's own rule, so the gateway's join produces one
			// separator rather than a doubled one.
			base_url: 'https://llm.example.com/v1'
		});
		await waitFor(() => expect(rendered.reloads()).toBe(1));
	});

	it('posts an Anthropic node with no api type at all', async () => {
		const stub = stubProviderNodes();
		renderSection(stub);

		await screen.getByRole('button', { name: 'Add Anthropic Compatible' }).click();
		await type('Name', 'Claude mirror');
		await type('Prefix', 'mirror');
		await type('Base URL', 'https://claude.example.com/v1');
		await screen.getByRole('button', { name: 'Add the provider' }).click();

		await waitFor(() => expect(stub.creates).toHaveLength(1));
		expect(stub.creates[0]).toEqual({
			name: 'Claude mirror',
			prefix: 'mirror',
			type: 'anthropic-compatible',
			base_url: 'https://claude.example.com/v1'
		});
	});

	// The reference's `Input` renders its `hint` prop under the field whatever the value is, and the value
	// is the vendor URL, so both are on screen at once. The panel shows the joined URL as well, and a
	// variant hint that a preview could hide would be the one piece of the reference's copy an operator
	// never reads.
	it('opens with the vendor URL as the value and the variant hint under the field', async () => {
		renderSection(stubProviderNodes());

		await screen.getByRole('button', { name: 'Add OpenAI Compatible' }).click();

		expect(value(screen.getByLabelText('Base URL'))).toBe('https://api.openai.com/v1');
		expect(squashed(screen.getByText(/Use the base URL/))).toContain(
			'Use the base URL (ending in /v1) for your OpenAI-compatible API.'
		);
	});

	it('states the Anthropic hint, which names the path the gateway appends', async () => {
		renderSection(stubProviderNodes());

		await screen.getByRole('button', { name: 'Add Anthropic Compatible' }).click();

		expect(value(screen.getByLabelText('Base URL'))).toBe('https://api.anthropic.com/v1');
		expect(squashed(screen.getByText(/Use the base URL/))).toContain(
			'The gateway appends /messages.'
		);
	});

	it('shows the URL the gateway will call while the base URL is typed', async () => {
		renderSection(stubProviderNodes());

		await screen.getByRole('button', { name: 'Add OpenAI Compatible' }).click();
		await type('Base URL', 'https://llm.example.com/v1');
		expect(squashed(screen.getByText(/The gateway will call/))).toContain(
			'https://llm.example.com/v1/chat/completions'
		);
	});

	it('puts the vendor URL back when the API type changes', async () => {
		renderSection(stubProviderNodes());

		await screen.getByRole('button', { name: 'Add OpenAI Compatible' }).click();
		await type('Base URL', 'https://llm.example.com/v1');
		await select('API type', 'responses');

		expect(value(screen.getByLabelText('Base URL'))).toBe('https://api.openai.com/v1');
	});

	it('refuses a prefix with a slash locally, and sends nothing', async () => {
		const stub = stubProviderNodes();
		renderSection(stub);

		await screen.getByRole('button', { name: 'Add OpenAI Compatible' }).click();
		await type('Name', 'Corp gateway');
		await type('Prefix', 'mycorp/prod');
		await type('Base URL', 'https://llm.example.com/v1');
		await screen.getByRole('button', { name: 'Add the provider' }).click();

		expect(
			screen.getByText('Use letters, digits, dots, dashes, and underscores only.')
		).toBeTruthy();
		expect(stub.creates).toHaveLength(0);
	});

	it('renders the gateway refusal and keeps the dialog open, naming the owner of a taken prefix', async () => {
		const stub = stubProviderNodes({ takenPrefixes: ['openai'] });
		renderSection(stub);

		await screen.getByRole('button', { name: 'Add OpenAI Compatible' }).click();
		await type('Name', 'Corp gateway');
		await type('Prefix', 'openai');
		await type('Base URL', 'https://llm.example.com/v1');
		await screen.getByRole('button', { name: 'Add the provider' }).click();

		await waitFor(() => expect(screen.getByRole('alert')).toBeTruthy());
		expect(squashed(screen.getByRole('alert'))).toContain(
			'prefix "openai" is already used by "openai"'
		);
		expect(screen.getByRole('heading', { name: 'Add OpenAI Compatible' })).toBeTruthy();
	});
});

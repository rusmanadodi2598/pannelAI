// A custom node's models section and its `/models` import (draft 019 F5, docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// A node has no registry catalog, so the rows this section lists ARE its models. That is why it states the
// string each row is addressed by and offers the reference's import
// (`providers/[id]/CompatibleModelsSection.js:125-187`). The cases here hold the node's copy, the addressed
// string with its copy control, the gate the reference puts on the import, and the four answers the import
// can give.

import { cleanup, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ProviderCustomModels from '$lib/components/ProviderCustomModels.svelte';
import { endpointRow } from '../support/endpoint-stub';
import { customRow, stubModels, type ModelStub } from '../support/model-stub';
import { squashed } from '../support/dom';

const NODE_ID = 'openai-compatible-01J';
const PREFIX = 'mycorp';

let stub: ModelStub;
let changed = 0;

beforeEach(() => {
	changed = 0;
	stub = stubModels({
		providers: ['openai', NODE_ID],
		endpoints: [endpointRow({ provider_id: NODE_ID })],
		custom: [
			customRow({ provider_id: NODE_ID, model_id: 'gpt-4o-mini', display_name: 'GPT-4o mini' })
		]
	});
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

function renderSection(): void {
	render(ProviderCustomModels, {
		props: {
			providerId: NODE_ID,
			nodePrefix: PREFIX,
			onchanged: () => {
				changed += 1;
			}
		}
	});
}

async function waitForTable(): Promise<HTMLElement> {
	return await screen.findByRole('table', { name: /custom models declared/i });
}

describe("a node's models section", () => {
	it('states the string each model is addressed by, with a copy control', async () => {
		renderSection();
		const table = await waitForTable();

		expect(within(table).getByText(`${PREFIX}/gpt-4o-mini`)).toBeTruthy();
		expect(within(table).getByRole('button', { name: 'Copy' })).toBeTruthy();
	});

	it('words its empty state for the node rather than for a supplement', async () => {
		stub.custom = [];
		renderSection();

		expect(await screen.findByText('No models for this node yet')).toBeTruthy();
		expect(squashed(screen.getByText(/A model declared here is addressed as/))).toContain(
			`${PREFIX}/model-id`
		);
	});
});

describe('importing from /models', () => {
	it('declares the models the upstream answers and skips the ones already declared', async () => {
		stub.providerModels = [{ id: 'gpt-4o-mini' }, { id: 'gpt-4o', name: 'GPT-4o' }];
		renderSection();
		await waitForTable();

		screen.getByRole('button', { name: 'Import from /models' }).click();

		await waitFor(() => expect(stub.customCreates).toHaveLength(1));
		expect(stub.providerModelsReads).toEqual([NODE_ID]);
		expect(stub.customCreates[0]).toEqual({
			provider_id: NODE_ID,
			model_id: 'gpt-4o',
			display_name: 'GPT-4o',
			capabilities: []
		});
		await waitFor(() => expect(screen.getByText('1 imported.')).toBeTruthy());
		await waitFor(() => expect(changed).toBe(1));
	});

	it('reports an answer with no models, and does not read as a failure', async () => {
		stub.providerModels = [];
		renderSection();
		await waitForTable();

		screen.getByRole('button', { name: 'Import from /models' }).click();

		await waitFor(() => expect(screen.getByText('No models returned from /models.')).toBeTruthy());
		expect(stub.customCreates).toEqual([]);
		expect(changed).toBe(0);
	});

	it('reports an answer whose models are all declared already', async () => {
		stub.providerModels = [{ id: 'gpt-4o-mini' }];
		renderSection();
		await waitForTable();

		screen.getByRole('button', { name: 'Import from /models' }).click();

		await waitFor(() => expect(screen.getByText('No new models were added.')).toBeTruthy());
		expect(stub.customCreates).toEqual([]);
	});

	it('reports a route that could not be read as the alert it is', async () => {
		stub.providerModelsReadStatus = 500;
		renderSection();
		await waitForTable();

		screen.getByRole('button', { name: 'Import from /models' }).click();

		const alert = await screen.findByRole('alert');
		expect(squashed(alert)).toContain('The models were not imported.');
	});

	it('renders the warning a list that is not the upstream answer carries', async () => {
		stub.providerModels = [{ id: 'gpt-4o' }];
		stub.providerModelsWarning =
			'The upstream could not be reached, so the registry list is shown.';
		renderSection();
		await waitForTable();

		screen.getByRole('button', { name: 'Import from /models' }).click();

		await waitFor(() =>
			expect(
				screen.getByText('The upstream could not be reached, so the registry list is shown.')
			).toBeTruthy()
		);
	});

	it('stays disabled until a connection that is not disabled exists', async () => {
		stub.endpoints = [endpointRow({ provider_id: NODE_ID, status: 'disabled' })];
		renderSection();
		await waitForTable();

		await waitFor(() =>
			expect(screen.getByText('Add a connection to enable importing models.')).toBeTruthy()
		);
		const button = screen.getByRole('button', { name: 'Import from /models' }) as HTMLButtonElement;
		expect(button.disabled).toBe(true);

		// A control that cannot act must not ask: the route is never called while the gate is shut.
		expect(stub.providerModelsReads).toEqual([]);
	});
});

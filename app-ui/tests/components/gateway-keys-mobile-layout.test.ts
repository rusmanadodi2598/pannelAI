// Endpoint & Key: the create form and the refresh control share one mobile width (antislop-layoutmobile,
// Form Row That Never Collapses, Action Pair, Sibling match).
//
// On a phone the name field, Create gateway key, and Refresh now are one column of equal-width rows at
// the full content width. From sm up the form returns to one line. A class-list test proves the rule was
// written; the browser pass measured the widths at 320, 360, 390, and 768.

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import GatewayKeysTab from '../../src/lib/components/GatewayKeysTab.svelte';
import { stubGatewayKeys } from '../support/gateway-key-stub';

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('the endpoint and key create form on mobile', () => {
	it('stacks the name field and Create gateway key as full-width rows, and returns inline from sm', async () => {
		stubGatewayKeys({ rows: [] });
		render(GatewayKeysTab);

		const input = screen.getByLabelText('Key name');
		const create = screen.getByRole('button', { name: 'Create gateway key' });
		const form = create.closest('form') as HTMLElement;

		// The field is full width on mobile, not a fixed 256px that refuses to shrink.
		expect(input.className).toContain('w-full');
		expect(input.className).not.toContain('w-64');
		expect(input.parentElement?.className).toContain('sm:w-64');

		// The form is a column on mobile and a wrapping row from sm up.
		expect(form.className).toContain('flex-col');
		expect(form.className).toContain('sm:flex-row');
		expect(form.className).toContain('items-stretch');

		// Create fills the row on mobile, and centres its label.
		for (const token of ['w-full', 'justify-center', 'min-h-11', 'sm:w-auto']) {
			expect(create.className, `Create gateway key lost ${token}`).toContain(token);
		}
	});

	it('gives Refresh now the same mobile width as the create row', async () => {
		stubGatewayKeys({ rows: [] });
		render(GatewayKeysTab);

		const refresh = await screen.findByRole('button', { name: 'Refresh now' });
		// The wrapper stretches the shared control to the row on mobile and releases it from sm up.
		const wrapper = refresh.closest('div')?.parentElement as HTMLElement;
		expect(wrapper.className).toContain('flex-col');
		expect(wrapper.className).toContain('sm:flex-row');
		expect(wrapper.className).toContain('[&_button]:w-full');
		expect(wrapper.className).toContain('sm:[&_button]:w-auto');
	});

	it('keeps the row rename field at touch height and inside its mobile card', async () => {
		stubGatewayKeys();
		render(GatewayKeysTab);

		const row = (await screen.findByText('Laptop')).closest('tr') as HTMLElement;
		await fireEvent.click(within(row).getByRole('button', { name: 'Rename' }));

		const input = screen.getByLabelText('New name');
		for (const token of ['min-h-11', 'w-48', 'max-w-full']) {
			expect(input.className, `the rename field lost ${token}`).toContain(token);
		}
	});
});

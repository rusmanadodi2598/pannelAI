// The §8.4.4 wiring on the settings-style forms that hold an unsaved draft.
//
// `tests/dirty-guard.test.ts` proves the registry and the cancel decision on their own. This file proves
// the other half of the rule for the five forms whose draft is compared against a settings document: the
// form that edits a draft actually registers it, and the registration follows the draft rather than the
// mount. Each row renders the real component with the fixture its own screen test uses, edits one field,
// asserts the guard answers "dirty", then puts the field back and asserts the answer follows. A form that
// registered once at mount and never looked again would fail the second assertion, and one that never
// registered would fail the first. The combo editor is the sixth form and its two cases live in
// `tests/components/dirty-combo-form.test.ts`, because its dirty state is a field-by-field comparison of
// the seeded form rather than a document comparison.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProxyOutboundSettings from '../../src/lib/components/ProxyOutboundSettings.svelte';
import SettingsLoggingTab from '../../src/lib/components/SettingsLoggingTab.svelte';
import SettingsRoutingTab from '../../src/lib/components/SettingsRoutingTab.svelte';
import SettingsSecurityTab from '../../src/lib/components/SettingsSecurityTab.svelte';
import TokenSaverForm from '../../src/lib/components/TokenSaverForm.svelte';
import { hasDirtyForm } from '../../src/lib/dirty-guard';
import {
	schemaLoggingSettingsForm,
	schemaRoutingSettingsForm,
	schemaSecuritySettingsForm
} from '$lib/schemas/settings';
import { schemaTokenSaver } from '$lib/schemas/token-saver';
import { routingFormDocument, settingsDocument } from '../support/settings-document';
import { stubProxies } from '../support/proxy-stub';
import { tokenSaverDocument } from '../support/token-saver-stub';
import { forEachCase } from '../support/tables';

const noop = async (): Promise<void> => {};

type FormCase = {
	name: string;
	/** Renders the form and waits until its draft is seeded, which the guard must see as clean. */
	mount: () => Promise<void>;
	/** One edit, the smallest change the operator can make. */
	edit: () => Promise<void>;
	/** The same edit undone, so the form is back to what it loaded. */
	undo: () => Promise<void>;
};

const CASES: FormCase[] = [
	{
		name: 'the Settings Security tab',
		mount: async () => {
			render(SettingsSecurityTab, {
				props: {
					loaded: schemaSecuritySettingsForm.parse(settingsDocument().security),
					onrefresh: noop
				}
			});
			await screen.findByRole('heading', { name: 'Access' });
		},
		edit: async () => {
			await fireEvent.click(
				screen.getByRole('checkbox', { name: /Require a gateway key on data plane requests/ })
			);
		},
		undo: async () => {
			await fireEvent.click(
				screen.getByRole('checkbox', { name: /Require a gateway key on data plane requests/ })
			);
		}
	},
	{
		name: 'the Settings Routing tab',
		mount: async () => {
			render(SettingsRoutingTab, {
				props: {
					loaded: schemaRoutingSettingsForm.parse(routingFormDocument()),
					onrefresh: noop
				}
			});
			await screen.findByRole('heading', { name: 'Routing' });
		},
		edit: async () => {
			await fireEvent.input(screen.getByLabelText('Routing sticky limit'), {
				target: { value: '9' }
			});
		},
		undo: async () => {
			await fireEvent.input(screen.getByLabelText('Routing sticky limit'), {
				target: { value: '3' }
			});
		}
	},
	{
		name: 'the Settings Logging tab',
		mount: async () => {
			render(SettingsLoggingTab, {
				props: {
					loaded: schemaLoggingSettingsForm.parse(settingsDocument().logging),
					onrefresh: noop
				}
			});
			await screen.findByRole('heading', { name: 'Logging' });
		},
		edit: async () => {
			await fireEvent.input(screen.getByLabelText(/Retention/), { target: { value: '30' } });
		},
		undo: async () => {
			await fireEvent.input(screen.getByLabelText(/Retention/), { target: { value: '7' } });
		}
	},
	{
		name: 'the Token Saver form',
		mount: async () => {
			render(TokenSaverForm, {
				props: { loaded: schemaTokenSaver.parse(tokenSaverDocument()), onrefresh: noop }
			});
			await screen.findByRole('heading', { name: 'RTK' });
		},
		edit: async () => {
			await fireEvent.click(screen.getByRole('checkbox', { name: /^grep/ }));
		},
		undo: async () => {
			await fireEvent.click(screen.getByRole('checkbox', { name: /^grep/ }));
		}
	},
	{
		name: 'the Proxy outbound card',
		mount: async () => {
			stubProxies();
			render(ProxyOutboundSettings);
			// The URL field only exists once the stored document has landed, and until it has, the card
			// has nothing to be dirty against.
			await screen.findByLabelText('Last-resort proxy URL');
		},
		edit: async () => {
			await fireEvent.input(screen.getByLabelText('Last-resort proxy URL'), {
				target: { value: 'http://proxy.internal:8080' }
			});
		},
		undo: async () => {
			await fireEvent.input(screen.getByLabelText('Last-resort proxy URL'), {
				target: { value: '' }
			});
		}
	}
];

describe('the drafted forms', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	forEachCase(CASES, async (testCase) => {
		await testCase.mount();

		// A form that has only loaded is not a draft: the guard must stay quiet until the operator edits.
		expect(hasDirtyForm()).toBe(false);

		await testCase.edit();
		await waitFor(() => expect(hasDirtyForm()).toBe(true));

		await testCase.undo();
		await waitFor(() => expect(hasDirtyForm()).toBe(false));
	});

	it('forgets a draft when its form unmounts, so a closed screen cannot block a navigation', async () => {
		render(SettingsRoutingTab, {
			props: {
				loaded: schemaRoutingSettingsForm.parse(routingFormDocument()),
				onrefresh: noop
			}
		});
		await screen.findByRole('heading', { name: 'Routing' });

		await fireEvent.input(screen.getByLabelText('Routing sticky limit'), {
			target: { value: '9' }
		});
		await waitFor(() => expect(hasDirtyForm()).toBe(true));

		cleanup();
		expect(hasDirtyForm()).toBe(false);
	});
});

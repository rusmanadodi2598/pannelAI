// DOM readers for the live panel's tests (src/lib/components/UsageLivePanel.svelte).
//
// The panel splits into two subjects that fail for different reasons: the row of connection controls at the
// top, and the Live activity tablist at the bottom. Both need the same handful of readers, and a second
// copy of `openPanel` is how one file starts asserting against the wrong tab panel, so they live here.
//
// Everything reads the panel the way a reader does: by its accessible name, and by clicking the tab it
// wants to see. `stubPanel` and `frame` from `live-panel-stub.ts` are the inputs; this is the output side.

import { screen } from '@testing-library/svelte';
import { frameText } from './live-stream';
import { frame, startedNow } from './live-panel-stub';

type Stream = { streams: { send: (text: string) => void }[] };

/** The connection chip is the row's first control, so it names the row for the tests. */
export async function liveRow(): Promise<HTMLElement> {
	const chip = await screen.findByText('Connecting');
	return chip.parentElement as HTMLElement;
}

/** The Live activity tablist, found by its accessible name. */
export function liveTabs(): HTMLElement {
	return screen.getByRole('tablist', { name: 'Live activity' });
}

/** Opens one tab of the Live activity tablist, the way a reader does: by clicking it. */
export async function openTab(name: RegExp): Promise<void> {
	const tab = await screen.findByRole('tab', { name });
	tab.click();
}

/** The panel of the tab that is open now. */
export function openPanel(): HTMLElement {
	const selected = screen.getByRole('tab', { selected: true });
	const panelId = selected.getAttribute('aria-controls');
	if (panelId === null) throw new Error('the selected tab names no panel');
	const panel = document.getElementById(panelId);
	if (panel === null) throw new Error('the selected tab panel is not in the document');
	return panel;
}

/** One active entry the staleness guard keeps, so the In flight tab carries it. */
export function sendInFlight(stub: Stream, providerId = 'openai'): void {
	stub.streams[0].send(
		frameText(
			frame({ active: [{ provider_id: providerId, model: 'gpt-4o', started_at: startedNow() }] })
		)
	);
}

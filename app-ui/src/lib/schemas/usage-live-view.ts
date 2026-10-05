// The Usage screen's live derivations (docs/DRAFT/012-USAGE-LIVE-UI-READINESS.md F2).
//
// Same rule as `usage-view.ts`: pure functions of their inputs, with no DOM, no network, and no clock of
// their own. The clock is a parameter wherever it matters, because the guard below is the one thing that
// decides whether the screen keeps claiming a provider is routing, and a rule about time has to be
// testable without waiting for it.
//
// Five derivations and one label table, and each exists because the wire's shape and the screen's question
// differ:
//
//   `liveMerge` folds a frame into the live state. The frame is full state for three fields and says
//   nothing about the rest, so the fold is where "the stream can never touch an aggregate" becomes
//   structural rather than a rule someone has to remember: there is no aggregate in this type.
//
//   `freshActive` drops in-flight entries that have outlived the guard, so a marker the gateway never
//   cleared stops lighting a node.
//
//   `streamStatus` decides what the connection label may say, which is what keeps the panel from labelling
//   a poll or a broken socket "Live" (R-36).
//
//   `liveFacts` says what the live row's facts tab may state, and `providerDisplayName` resolves the ids
//   it names. Together they carry the sentence that left the drawing's frame (owner's correction,
//   2026-09-27, draft 035 F2).
//
// The drawing's own derivations live in `usage-topology-view.ts`.

import type { UsageLiveActive, UsageLiveFrame, UsageLiveRecent } from './usage-live';

/**
 * How long an in-flight entry stays on screen after the instant it says it started.
 *
 * The reference fork keeps the same figure (`ProviderTopology.js`, `FE_ACTIVE_TIMEOUT_MS`), and the reason
 * it exists is that a gateway which dies mid-request leaves its marker behind: the frame that would have
 * cleared it never arrives, so without a guard the screen would light that node forever.
 */
export const FE_ACTIVE_TIMEOUT_MS = 60_000;

/** The live half of the Usage overview. Every field here comes from the stream and nowhere else. */
export type LiveView = {
	active: UsageLiveActive[];
	recent: UsageLiveRecent[];
	/** The provider the gateway last reported an error for, or an empty string for none. */
	errorProvider: string;
	/** When the frame this state came from landed, on the caller's clock. */
	receivedAt: number;
};

/**
 * Folds one frame into the live state.
 *
 * A field the frame does not carry leaves the previous value alone rather than clearing it: the frame is
 * the gateway's statement about what it is doing now, and a key it did not send is not a statement that
 * nothing is happening. The staleness guard is what stops a gateway that has quietly stopped sending
 * `active` from keeping nodes lit.
 */
export function liveMerge(
	previous: LiveView | null,
	frame: UsageLiveFrame,
	receivedAt: number
): LiveView {
	return {
		active: frame.active ?? previous?.active ?? [],
		recent: frame.recent ?? previous?.recent ?? [],
		errorProvider: frame.error_provider ?? previous?.errorProvider ?? '',
		receivedAt
	};
}

/**
 * The in-flight entries the screen still trusts.
 *
 * The guard reads `started_at` rather than the instant the panel first saw the entry, and that choice is
 * the point: a gateway that keeps repeating a stuck request would refresh the panel's own timestamp on
 * every frame, and the entry would never age out.
 *
 * An entry whose start instant is in the future is kept. That is a gateway clock ahead of the browser's,
 * and the panel has no basis for calling a request stale on arithmetic it cannot verify.
 */
export function freshActive(
	active: UsageLiveActive[],
	now: number,
	timeoutMs = FE_ACTIVE_TIMEOUT_MS
): UsageLiveActive[] {
	return active.filter((entry) => now - Date.parse(entry.started_at) < timeoutMs);
}

export type StreamStatus = 'idle' | 'connecting' | 'live' | 'paused' | 'unavailable';

export type StreamInput = {
	/** The operator paused the stream. */
	paused: boolean;
	/** A frame has arrived on the connection that is open now. */
	fresh: boolean;
	/** The last attempt failed and no frame has arrived since. */
	down: boolean;
	/** A connection is open, or an attempt is scheduled. */
	active: boolean;
};

/**
 * What the connection label may say.
 *
 * `live` requires a frame on the connection that is open *now*, not a frame that arrived at some point.
 * That is the whole rule R-36 asks for: a socket that died a minute ago, or one that has not yet answered,
 * must not be labelled with the same word as one that is delivering data.
 *
 * A pause outranks a failure because while the operator has paused it, the reason the panel is not live is
 * the pause, which is something they did rather than something that went wrong.
 */
export function streamStatus(input: StreamInput): StreamStatus {
	if (input.paused) return 'paused';
	if (input.down) return 'unavailable';
	if (!input.active) return 'idle';
	return input.fresh ? 'live' : 'connecting';
}

const STREAM_LABELS: Record<StreamStatus, string> = {
	idle: 'Not running',
	connecting: 'Connecting',
	live: 'Live',
	paused: 'Paused',
	unavailable: 'Unavailable'
};

/**
 * The label for a connection status. Only `live` may be called Live (R-36), which is why the mapping is a
 * table here rather than a string built where the label is drawn.
 */
export function streamLabel(status: StreamStatus): string {
	return STREAM_LABELS[status];
}

/** A provider row as the panel's registry read carries it: an id and the name to draw. */
export type ProviderRow = { id: string; name: string };

/**
 * One live fact as the row states it: the label names the state, the value names the providers, and the
 * tone is the drawing's own colour rule, `status` for what is routing and plain for what merely happened
 * (draft 023 F1, which the move to the row kept).
 */
export type LiveFact = {
	label: string;
	value: string;
	tone: 'status' | 'plain';
};

/** A provider's display name, or its id when the registry has no entry for it. */
export function providerDisplayName(providers: ProviderRow[], id: string): string {
	const match = providers.find((provider) => provider.id.toLowerCase() === id.toLowerCase());
	return match?.name ?? id;
}

/**
 * The combo names with a request in flight, in the order the frame reports them, without repeats.
 *
 * These are the ids the drawing lights on its combo band. They are compared exactly and never resolved
 * through `providerDisplayName`, because a combo name is an identifier the operator typed and the gateway
 * wrote back: a combo named `openai` is a combo, not the OpenAI provider, and translating it would light
 * the wrong fact.
 */
export function activeComboNames(active: UsageLiveActive[]): string[] {
	const names: string[] = [];
	for (const entry of active) {
		const combo = entry.combo?.trim() ?? '';
		if (combo === '' || names.includes(combo)) continue;
		names.push(combo);
	}
	return names;
}

/**
 * The in-flight requests that addressed no combo.
 *
 * This is what the drawing's direct hop carries: a request that entered through a combo travels
 * client → combo → gateway, and a beam on the direct line would claim a request that never took it.
 */
export function directCount(active: UsageLiveActive[]): number {
	return active.filter((entry) => (entry.combo?.trim() ?? '') === '').length;
}

/** One in-flight entry as the facts row states it: the combo it entered through, then who answered it. */
function describeEntry(providers: ProviderRow[], entry: UsageLiveActive): string {
	const name = providerDisplayName(providers, entry.provider_id);
	// A frame may carry model as an empty string, and ' ()' after a name is a typo the
	// wire should not be allowed to put on the screen.
	const model = entry.model?.trim() ?? '';
	const answered = model === '' ? name : `${name} (${model})`;
	const combo = entry.combo?.trim() ?? '';
	return combo === '' ? answered : `${combo} → ${answered}`;
}

/**
 * The live facts that are happening, in the order the stream reports them.
 *
 * Absence is not a fact: a screen with nothing in flight and nothing finished states none of these, and
 * the connection chip is what says what the state is (owner's correction, 2026-09-23, carried with the
 * sentence when it moved out of the drawing's frame and into the row, draft 035 F2). A value carries no
 * sentence-final period because the tab reads as a label, not as a paragraph.
 *
 * The combo is stated because the drawing encodes it: the drawing is hidden from assistive technology, and
 * the row is what says in words what the nodes and edges claim (SPEC-UI §6.5).
 */
export function liveFacts(
	providers: ProviderRow[],
	active: UsageLiveActive[],
	last: string,
	error: string
): LiveFact[] {
	const facts: LiveFact[] = [];

	if (active.length > 0) {
		facts.push({
			label: `${active.length} in flight`,
			value: active.map((entry) => describeEntry(providers, entry)).join(', '),
			tone: 'status'
		});
	}

	if (last !== '') {
		facts.push({
			label: 'Last finished',
			value: providerDisplayName(providers, last),
			tone: 'plain'
		});
	}

	if (error !== '') {
		facts.push({
			label: 'Last error',
			value: providerDisplayName(providers, error),
			tone: 'plain'
		});
	}

	return facts;
}

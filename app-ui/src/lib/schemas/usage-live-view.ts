// The Usage screen's live derivations (docs/DRAFT/012-USAGE-LIVE-UI-READINESS.md F2).
//
// Same rule as `usage-view.ts`: pure functions of their inputs, with no DOM, no network, and no clock of
// their own. The clock is a parameter wherever it matters, because the guard below is the one thing that
// decides whether the screen keeps claiming a provider is routing, and a rule about time has to be
// testable without waiting for it.
//
// Three functions, and each exists because the wire's shape and the screen's question differ:
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

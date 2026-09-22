// Fixture and fake API for the live panel's tests (src/lib/components/UsageLivePanel.svelte).
//
// The panel makes two reads and neither is the other's stand-in: the registry, which the drawing is built
// from, and the live stream, which the activity comes from. One stub answers both and records them apart,
// because a test that read "the last request" would not know which read it was looking at.
//
// The live answers are a list rather than one response: a `Response` wraps a stream once, so a test about
// recovery has to hand the second attempt a body of its own. `liveStreams()` builds those.

import { vi } from 'vitest';
import { liveStreams, type LiveAnswer, type LiveBody } from './live-stream';
import { provider } from './providers-route-stub';

export function frame(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return { active: [], recent: [], error_provider: '', ...overrides };
}

/** A live entry that started just now, so the staleness guard keeps it. */
export function startedNow(): string {
	return new Date().toISOString();
}

export type PanelOptions = {
	providers?: Record<string, unknown>[];
	total?: number;
	providersStatus?: number;
	providersMessage?: string;
	/** The answers for the live route, in order. The last one repeats. */
	liveAnswers?: LiveAnswer[];
};

export type PanelStub = {
	/** Every request the panel made, in order. */
	queries: string[];
	/** The streams the live answers built, in the order they were asked for. */
	streams: LiveBody[];
	/** The init each live request was sent with, so an abort can be told from a close. */
	liveInits: (RequestInit | undefined)[];
	/** How many requests went to the live route. */
	liveCalls: () => number;
};

export function stubPanel(options: PanelOptions = {}): PanelStub {
	const queries: string[] = [];
	const liveInits: (RequestInit | undefined)[] = [];
	const { answer, streams } = liveStreams();
	const answers = options.liveAnswers ?? [answer];
	const rows = options.providers ?? [provider({ id: 'openai', name: 'OpenAI', endpoint_count: 2 })];

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const url = String(input);
		queries.push(url);

		if (url.includes('/usage/live')) {
			liveInits.push(init);
			const index = Math.min(liveInits.length - 1, answers.length - 1);
			return answers[index]();
		}

		if (options.providersStatus && options.providersStatus !== 200) {
			return new Response(
				JSON.stringify({
					error: { code: 'INTERNAL_ERROR', message: options.providersMessage ?? 'boom' }
				}),
				{ status: options.providersStatus, headers: { 'content-type': 'application/json' } }
			);
		}

		return new Response(
			JSON.stringify({
				data: rows,
				meta: { page: 1, per_page: 100, total: options.total ?? rows.length }
			}),
			{ status: 200, headers: { 'content-type': 'application/json' } }
		);
	});

	return {
		queries,
		streams,
		liveInits,
		liveCalls: () => queries.filter((url) => url.includes('/usage/live')).length
	};
}

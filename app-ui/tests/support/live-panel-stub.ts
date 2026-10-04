// Fixture and fake API for the live panel's tests (src/lib/components/UsageLivePanel.svelte).
//
// The panel makes three reads and none is another's stand-in: the registry, which the drawing's upstream
// nodes come from, the combo list, which its combo band comes from, and the live stream, which the activity
// comes from. One stub answers all three and records them apart, because a test that read "the last request"
// would not know which read it was looking at.
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

/**
 * One stored combo, as §7.7 returns it.
 *
 * The panel's schema requires the strategy, the sticky limit and the model list, so a row that left them
 * out would fail to parse and the panel would report a failed read rather than draw the band.
 */
export function comboRow(name: string): Record<string, unknown> {
	return {
		id: `cmb_${name}`,
		name,
		strategy: 'fallback',
		sticky_limit: 1,
		judge_model: '',
		models: [{ ref: 'openai/gpt-4o', priority: 0 }],
		created_at: '2026-09-22T10:00:00Z',
		updated_at: '2026-09-22T10:00:00Z'
	};
}

export type PanelOptions = {
	providers?: Record<string, unknown>[];
	total?: number;
	providersStatus?: number;
	providersMessage?: string;
	combos?: Record<string, unknown>[];
	combosTotal?: number;
	combosStatus?: number;
	combosMessage?: string;
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
	const comboRows = options.combos ?? [comboRow('pro-tier')];

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const url = String(input);
		queries.push(url);

		if (url.includes('/usage/live')) {
			liveInits.push(init);
			const index = Math.min(liveInits.length - 1, answers.length - 1);
			return answers[index]();
		}

		if (url.includes('/combos')) {
			if (options.combosStatus && options.combosStatus !== 200) {
				return errorResponse(
					options.combosStatus,
					options.combosMessage ?? 'the combo list is unreachable'
				);
			}
			return listResponse(comboRows, options.combosTotal ?? comboRows.length);
		}

		if (options.providersStatus && options.providersStatus !== 200) {
			return errorResponse(options.providersStatus, options.providersMessage ?? 'boom');
		}

		return listResponse(rows, options.total ?? rows.length);
	});

	return {
		queries,
		streams,
		liveInits,
		liveCalls: () => queries.filter((url) => url.includes('/usage/live')).length
	};
}

function listResponse(data: Record<string, unknown>[], total: number): Response {
	return new Response(JSON.stringify({ data, meta: { page: 1, per_page: 100, total } }), {
		status: 200,
		headers: { 'content-type': 'application/json' }
	});
}

function errorResponse(status: number, message: string): Response {
	return new Response(JSON.stringify({ error: { code: 'INTERNAL_ERROR', message } }), {
		status,
		headers: { 'content-type': 'application/json' }
	});
}

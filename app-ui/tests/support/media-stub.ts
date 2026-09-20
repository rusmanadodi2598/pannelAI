// Shared fixture and fake API for the Media Provider screen tests (docs/SPEC-UI/001-SPEC-UI.md §6.8).
//
// The stub keeps the rows and applies every save to them, resolving each field the way the server
// does: an empty override falls back to what the registry declares, and the source reports which of
// the two won. A stub that echoed the body back would let a card that never re-reads pass, and it
// would also hide the one rule this screen exists to get right.
//
// It also refuses the save the server refuses, with the server's own sentence, so a test can prove
// the panel's block and the API's refusal say the same thing rather than two similar things.
//
// Reading a control's state is `tests/support/dom.ts`, which the component tests share.

import { vi } from 'vitest';

export function mediaRow(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		provider_id: 'openai',
		provider_name: 'OpenAI',
		kind: 'tts',
		base_url: 'https://api.openai.com/v1/audio/speech',
		base_url_source: 'registry',
		default_model: 'tts-1',
		default_model_source: 'registry',
		endpoint_count: 2,
		models: [{ id: 'tts-1', name: 'TTS 1' }, { id: 'tts-1-hd' }],
		...overrides
	};
}

export type MediaStub = {
	rows: Record<string, unknown>[];
	/** What the registry declares per provider, which is what clearing an override resolves to. */
	registry: Record<string, { base_url: string; default_model: string }>;
	patches: { id: string; body: Record<string, unknown> }[];
	reads: string[];
	readStatus: number;
	writeStatus: number;
};

/** What a row's own values tell us the registry declares, for the fields the registry is serving. */
function declaredFrom(row: Record<string, unknown>): { base_url: string; default_model: string } {
	return {
		base_url: row.base_url_source === 'registry' ? String(row.base_url) : '',
		default_model: row.default_model_source === 'registry' ? String(row.default_model) : ''
	};
}

export function stubMediaProviders(overrides: Partial<MediaStub> = {}): MediaStub {
	const rows = overrides.rows ?? [];
	const registry: Record<string, { base_url: string; default_model: string }> = {};

	for (const row of rows) {
		const id = String(row.provider_id);
		// A caller's explicit value wins, so a test can model an override whose registry value differs.
		registry[id] = overrides.registry?.[id] ?? declaredFrom(row);
	}
	for (const [id, value] of Object.entries(overrides.registry ?? {})) registry[id] = value;

	const stub: MediaStub = {
		rows,
		registry,
		patches: [],
		reads: [],
		readStatus: 200,
		writeStatus: 200,
		...overrides
	};
	stub.rows = rows;
	stub.registry = registry;

	const json = (payload: unknown, status = 200): Response =>
		new Response(JSON.stringify(payload), {
			status,
			headers: { 'content-type': 'application/json' }
		});
	const refusal = (code: string, message: string, status: number): Response =>
		json({ error: { code, message } }, status);

	/** The patch answer is one kind block, without the provider's identity. */
	function blockOf(row: Record<string, unknown>): Record<string, unknown> {
		const { provider_id: _id, provider_name: _name, ...block } = row;
		return block;
	}

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const method = init?.method ?? 'GET';
		const url = String(input);
		const body: Record<string, unknown> =
			init?.body === undefined ? {} : JSON.parse(String(init.body));

		const byId = /\/media-providers\/([^/?]+)$/.exec(url);
		if (method === 'PATCH' && byId) {
			const id = decodeURIComponent(byId[1]);
			stub.patches.push({ id, body });

			if (stub.writeStatus !== 200) {
				return refusal('VALIDATION_ERROR', 'The gateway refused this value.', stub.writeStatus);
			}

			const row = stub.rows.find((entry) => entry.provider_id === id && entry.kind === body.kind);
			if (!row) return refusal('NOT_FOUND', 'provider is not in the registry', 404);

			const declared = stub.registry[id] ?? { base_url: '', default_model: '' };
			const nextBaseUrl = String(body.base_url ?? '').trim();

			// The server's own rule, quoted so the two cannot drift: a save that leaves no base URL from
			// either source is refused, and nothing else is.
			if (nextBaseUrl === '' && declared.base_url === '') {
				return refusal(
					'VALIDATION_ERROR',
					`provider ${id} has no ${String(body.kind)} base_url; set one`,
					400
				);
			}

			const nextModel = String(body.default_model ?? '').trim();

			row.base_url = nextBaseUrl === '' ? declared.base_url : nextBaseUrl;
			row.base_url_source = nextBaseUrl === '' ? 'registry' : 'override';
			row.default_model = nextModel === '' ? declared.default_model : nextModel;
			row.default_model_source = nextModel === '' ? 'registry' : 'override';

			return json(blockOf(row));
		}

		if (url.includes('/media-providers')) {
			// Recorded before the status is checked, because the list is of attempts: a test that counts
			// reads to prove a retry happened needs the failed one counted too.
			const kind = new URL(url, 'http://panel.test').searchParams.get('kind');
			stub.reads.push(String(kind));

			if (stub.readStatus !== 200) {
				return refusal('INTERNAL_ERROR', 'The registry is unreachable.', stub.readStatus);
			}

			return json({ data: stub.rows.filter((row) => kind === null || row.kind === kind) });
		}

		return refusal('NOT_FOUND', 'No route matches that request.', 404);
	});

	return stub;
}

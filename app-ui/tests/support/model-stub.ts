// Shared fake API for the provider detail model writes (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The stub applies every write to its own state and answers the way the server does, because the two
// rules this slice turns on are rules about state: the merged catalog EXCLUDES a disabled model, and
// `PUT /models/disabled` replaces the WHOLE set. A stub that echoed the body back would let a panel that
// sends one provider's slice pass, and that panel would erase every other provider's rows in production.
// The alias set is replaced whole for the same reason, so it is faked the same way.
//
// It refuses what the server refuses, with the server's own sentences: `unknown model: <key>` for a
// disabled pair the catalog does not hold, `unknown provider_id: <id>` for a custom row under a provider
// the registry does not carry, a 409 for a pair that is already declared, `alias <name> is already a combo
// name` and `alias <name> targets an unknown model or combo: <target>` for an alias write.

import { vi } from 'vitest';

export type StubModel = Record<string, unknown>;

export type ModelStub = {
	/** The merged catalog, as the registry plus custom rows would produce it. */
	catalog: StubModel[];
	/** The disabled set, every provider's pairs. */
	disabled: StubModel[];
	/** The custom rows, every provider's. */
	custom: StubModel[];
	/** The alias set, every alias the gateway resolves. */
	aliases: StubModel[];
	/** The combos, whose names are a legal alias target. */
	combos: StubModel[];
	providers: string[];
	disabledWrites: StubModel[][];
	customCreates: StubModel[];
	customDeletes: string[];
	aliasWrites: StubModel[][];
	reads: string[];
	readStatus: number;
	/** The disabled set's own status, for a test that needs one read to fail and another to succeed. */
	disabledReadStatus: number;
	/** The alias set's own status, for the same reason. */
	aliasReadStatus: number;
	/** What `GET /combos` reports as the total, when a test needs the page to look truncated. */
	combosTotal: number | null;
	writeStatus: number;
};

export function catalogRow(overrides: StubModel = {}): StubModel {
	return {
		id: 'openai/gpt-4o',
		provider_id: 'openai',
		model_id: 'gpt-4o',
		display_name: 'GPT-4o',
		kind: 'chat',
		capabilities: ['vision'],
		source: 'registry',
		...overrides
	};
}

export function comboRow(overrides: StubModel = {}): StubModel {
	return {
		id: 'cmb_01',
		name: 'fallback-combo',
		strategy: 'fallback',
		sticky_limit: 0,
		judge_model: '',
		models: [],
		created_at: '2026-09-20T03:00:00Z',
		updated_at: '2026-09-20T03:00:00Z',
		...overrides
	};
}

export function aliasRow(overrides: StubModel = {}): StubModel {
	return { alias: 'fast', target: 'openai/gpt-4o', ...overrides };
}

export function customRow(overrides: StubModel = {}): StubModel {
	return {
		id: 'mdl_01',
		provider_id: 'openai',
		model_id: 'gpt-4o-mini',
		display_name: 'GPT-4o mini',
		capabilities: ['tools'],
		created_at: '2026-09-20T03:00:00Z',
		...overrides
	};
}

/** The provider detail body the page loads before any of the model sections render. */
export function providerDetailRow(overrides: StubModel = {}): StubModel {
	return {
		id: 'openai',
		name: 'OpenAI',
		category: 'apikey',
		auth_type: 'bearer',
		auth_modes: ['api_key'],
		has_oauth: false,
		no_auth: false,
		routability: 'native',
		endpoint_count: 0,
		status_summary: { total: 0, active: 0, disabled: 0, error: 0, rate_limited: 0 },
		base_url: 'https://api.openai.com/v1',
		format: 'openai',
		url_suffix: '',
		validate_url: '',
		timeout_ms: 30000,
		model_count: 2,
		chat_model_count: 2,
		media: [],
		deprecated: false,
		...overrides
	};
}

export function stubModels(overrides: Partial<ModelStub> = {}): ModelStub {
	const stub: ModelStub = {
		catalog: [],
		disabled: [],
		custom: [],
		aliases: [],
		combos: [],
		providers: ['openai', 'anthropic'],
		disabledWrites: [],
		customCreates: [],
		customDeletes: [],
		aliasWrites: [],
		reads: [],
		readStatus: 200,
		disabledReadStatus: 200,
		aliasReadStatus: 200,
		combosTotal: null,
		writeStatus: 200,
		...overrides
	};

	const json = (payload: unknown, status = 200): Response =>
		new Response(JSON.stringify(payload), {
			status,
			headers: { 'content-type': 'application/json' }
		});
	const refusal = (code: string, message: string, status: number): Response =>
		json({ error: { code, message } }, status);

	const key = (ref: StubModel): string => `${ref.provider_id}/${ref.model_id}`;
	const blocked = (): Set<string> => new Set(stub.disabled.map(key));

	// A custom row is a catalog row with `source: custom`, which is what makes an add change the catalog.
	function catalogWithCustom(): StubModel[] {
		const merged = new Map<string, StubModel>();
		for (const row of stub.catalog) merged.set(key(row), row);
		for (const row of stub.custom) {
			merged.set(key(row), { ...row, kind: undefined, source: 'custom' });
		}
		return [...merged.values()];
	}

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const method = init?.method ?? 'GET';
		const url = String(input);
		const body: StubModel = init?.body === undefined ? {} : JSON.parse(String(init.body));
		const parsed = new URL(url, 'http://panel.test');

		// The page's own load, and the endpoint list its last section reads. Both answer the shape the
		// panel's schemas expect, so a test can render the whole screen and assert on one section.
		const providerMatch = /\/providers\/([^/?]+)$/.exec(parsed.pathname);
		if (method === 'GET' && providerMatch) {
			return json(providerDetailRow({ id: decodeURIComponent(providerMatch[1]) }));
		}
		if (method === 'GET' && parsed.pathname.endsWith('/endpoints')) {
			return json({ data: [], meta: { page: 1, per_page: 25, total: 0 } });
		}

		if (parsed.pathname.endsWith('/models/disabled')) {
			if (method === 'PUT') {
				const models = (body.models ?? []) as StubModel[];
				stub.disabledWrites.push(models);

				if (stub.writeStatus !== 200) {
					return refusal('INTERNAL_ERROR', 'The set could not be stored.', stub.writeStatus);
				}

				const available = new Set(catalogWithCustom().map(key));
				for (const ref of models) {
					if (!available.has(key(ref))) {
						return refusal('VALIDATION_ERROR', `unknown model: ${key(ref)}`, 400);
					}
				}

				stub.disabled = models.map((ref) => ({ ...ref }));
				return json({ data: stub.disabled });
			}

			stub.reads.push('disabled');
			if (stub.disabledReadStatus !== 200) {
				return refusal('INTERNAL_ERROR', 'The set could not be read.', stub.disabledReadStatus);
			}
			return json({ data: stub.disabled });
		}

		if (parsed.pathname.endsWith('/models/custom')) {
			if (method === 'POST') {
				stub.customCreates.push(body);

				if (stub.writeStatus !== 200) {
					return refusal('INTERNAL_ERROR', 'The row could not be stored.', stub.writeStatus);
				}

				const providerId = String(body.provider_id ?? '');
				if (!stub.providers.includes(providerId)) {
					return refusal('VALIDATION_ERROR', `unknown provider_id: ${providerId}`, 400);
				}
				if (catalogWithCustom().some((row) => key(row) === `${providerId}/${body.model_id}`)) {
					return refusal('CONFLICT', 'model already exists: models_custom_pair', 409);
				}

				const row = customRow({
					id: `mdl_${String(stub.custom.length + 1).padStart(2, '0')}`,
					provider_id: providerId,
					model_id: body.model_id,
					display_name: body.display_name,
					capabilities: body.capabilities ?? []
				});
				stub.custom = [row, ...stub.custom];
				return json(row, 201);
			}

			stub.reads.push('custom');
			if (stub.readStatus !== 200) {
				return refusal('INTERNAL_ERROR', 'The rows could not be read.', stub.readStatus);
			}
			return json({ data: stub.custom });
		}

		const deleteMatch = /\/models\/custom\/([^/?]+)$/.exec(parsed.pathname);
		if (method === 'DELETE' && deleteMatch) {
			const id = decodeURIComponent(deleteMatch[1]);
			stub.customDeletes.push(id);

			if (stub.writeStatus !== 200) {
				return refusal('INTERNAL_ERROR', 'The row could not be removed.', stub.writeStatus);
			}

			const row = stub.custom.find((entry) => entry.id === id);
			if (!row) return refusal('NOT_FOUND', 'model is not in the catalog', 404);

			stub.custom = stub.custom.filter((entry) => entry.id !== id);
			return new Response(null, { status: 204 });
		}

		if (parsed.pathname.endsWith('/models/aliases')) {
			if (method === 'PUT') {
				const entries = (body.aliases ?? []) as StubModel[];
				stub.aliasWrites.push(entries);

				if (stub.writeStatus !== 200) {
					return refusal('INTERNAL_ERROR', 'The set could not be stored.', stub.writeStatus);
				}

				const comboNames = new Set(stub.combos.map((combo) => String(combo.name)));
				const available = new Set(catalogWithCustom().map(key));
				for (const entry of entries) {
					const name = String(entry.alias ?? '');
					const target = String(entry.target ?? '');
					if (comboNames.has(name)) {
						return refusal('VALIDATION_ERROR', `alias ${name} is already a combo name`, 400);
					}
					if (available.has(target) || comboNames.has(target)) continue;
					return refusal(
						'VALIDATION_ERROR',
						`alias ${name} targets an unknown model or combo: ${target}`,
						400
					);
				}

				// The server answers with the set as sent, which is why the panel sorts the body: the answer
				// is what the table renders until the next read.
				stub.aliases = entries.map((entry) => ({ ...entry }));
				return json({ data: stub.aliases });
			}

			stub.reads.push('aliases');
			if (stub.aliasReadStatus !== 200) {
				return refusal('INTERNAL_ERROR', 'The set could not be read.', stub.aliasReadStatus);
			}
			return json({ data: stub.aliases });
		}

		if (method === 'GET' && parsed.pathname.endsWith('/combos')) {
			stub.reads.push('combos');

			const page = Number(parsed.searchParams.get('page') ?? 1);
			const perPage = Number(parsed.searchParams.get('per_page') ?? 25);
			const start = (page - 1) * perPage;
			return json({
				data: stub.combos.slice(start, start + perPage),
				meta: {
					page,
					per_page: perPage,
					total: stub.combosTotal ?? stub.combos.length
				}
			});
		}

		if (parsed.pathname.endsWith('/models/catalog')) {
			stub.reads.push(`catalog:${parsed.searchParams.get('provider_id') ?? ''}`);

			if (stub.readStatus !== 200) {
				return refusal('INTERNAL_ERROR', 'The catalog could not be read.', stub.readStatus);
			}

			const providerId = parsed.searchParams.get('provider_id') ?? '';
			const capability = parsed.searchParams.get('capability') ?? '';
			const query = (parsed.searchParams.get('q') ?? '').toLowerCase();
			const hidden = blocked();

			// The merged view, with disabled rows removed: the server hides a disabled pair from both its
			// halves, so a disabled custom row is hidden too.
			const answer = catalogWithCustom().filter((row) => {
				if (hidden.has(key(row))) return false;
				if (providerId !== '' && row.provider_id !== providerId) return false;
				if (capability !== '' && !((row.capabilities ?? []) as string[]).includes(capability)) {
					return false;
				}
				if (query !== '' && !`${row.model_id} ${row.display_name}`.toLowerCase().includes(query)) {
					return false;
				}
				return true;
			});

			return json({ data: answer });
		}

		return refusal('NOT_FOUND', 'No route matches that request.', 404);
	});

	return stub;
}

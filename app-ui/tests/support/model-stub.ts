// Shared fake API for the provider detail model writes (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The stub applies every write to its own state and answers the way the server does, because the two
// rules this slice turns on are rules about state: the merged catalog EXCLUDES a disabled model, and
// `PUT /models/disabled` replaces the WHOLE set. A stub that echoed the body back would let a panel that
// sends one provider's slice pass, and that panel would erase every other provider's rows in production.
//
// It refuses what the server refuses, with the server's own sentences: `unknown model: <key>` for a
// disabled pair the catalog does not hold, `unknown provider_id: <id>` for a custom row under a provider
// the registry does not carry, and a 409 for a pair that is already declared.
//
// It also answers the two endpoint routes the detail page's key dialog turns on: the list the Endpoints
// section reads, and the create it posts to. That is what lets a test render the whole screen and hold the
// page's own wiring (the button, the token bump) rather than the dialog on its own.
//
// It serves the provider list too, because the combo editor's picker reads it for its active set: the rows
// are the test's (`providerRows`), so a picker test says which providers are configured rather than
// inheriting the catalog's.

import { vi } from 'vitest';
import { endpointRow } from './endpoint-stub';
import { settingsDocument } from './settings-document';

export type StubModel = Record<string, unknown>;

export type ModelStub = {
	/** The merged catalog, as the registry plus custom rows would produce it. */
	catalog: StubModel[];
	/** The disabled set, every provider's pairs. */
	disabled: StubModel[];
	/** The custom rows, every provider's. */
	custom: StubModel[];
	/** The combos, whose names are a legal reference. */
	combos: StubModel[];
	/** The probe results `POST /combos/{id}/test` answers, one entry per stored reference. */
	comboTestResults: StubModel[];
	/** The test route's own status, for a test that needs the combo to look deleted. */
	comboTestStatus: number;
	/** The combo ids the test route was called with. */
	comboTests: string[];
	/** Whether the provider detail says this provider has OAuth, which is what renders the section. */
	hasOAuth: boolean;
	/**
	 * The flow `GET /oauth/status` reports, which is what decides what the section offers. The default is
	 * `code`, the one flow that offers the start action, because no live provider reports it today
	 * (SPEC-UI §14 Q23); a test that needs the dormant case sets `device` or `connector`.
	 */
	oauthFlow: string;
	/** The connected accounts the status route answers. */
	oauthEndpoints: StubModel[];
	/** The status route's own status, for a test that needs one read to fail. */
	oauthReadStatus: number;
	/** When set, the start route answers this refusal instead of an authorize URL. */
	oauthStartRefusal: { status: number; code: string; message: string } | null;
	/** When set, the refresh route answers this refusal instead of moving a token. */
	oauthRefreshRefusal: { status: number; code: string; message: string } | null;
	/** The provider ids the start route was called with. */
	oauthStarts: string[];
	/** The endpoint ids the refresh route was called with, `null` for "every due account". */
	oauthRefreshes: (string | null)[];
	/** The provider ids the custom-model route knows, which is what makes an unknown one a refusal. */
	providers: string[];
	/**
	 * The provider list's rows, which the picker reads for its active set and its group names. Empty by
	 * default: a test that opens a picker sets the rows it wants offered.
	 */
	providerRows: StubModel[];
	disabledWrites: StubModel[][];
	customCreates: StubModel[];
	customDeletes: string[];
	reads: string[];
	readStatus: number;
	/** The disabled set's own status, for a test that needs one read to fail and another to succeed. */
	disabledReadStatus: number;
	/** What `GET /combos` reports as the total, when a test needs the page to look truncated. */
	combosTotal: number | null;
	writeStatus: number;
	/** The provider's own auth type, which is what decides whether the page offers the key dialog. */
	authType: string;
	/** The endpoint rows the Endpoints section lists. */
	endpoints: StubModel[];
	/**
	 * The connections list's own status, for a test that needs the label read to fail while the rest of the
	 * page loads. It is the same route, so the section behind the dialog reports the failure too.
	 */
	endpointReadStatus: number;
	/** The bodies `POST /endpoints` received, which is where a key name the route would refuse shows up. */
	endpointCreates: StubModel[];
	/** The bodies `POST /endpoints/bulk` received, one per paste the screen sent as a batch. */
	endpointBulkCreates: StubModel[];
	/** The models `GET /providers/{id}/models` answers, which is what a custom node's import declares. */
	providerModels: StubModel[];
	/** The route's own status, for a test that needs the import's read to fail. */
	providerModelsReadStatus: number;
	/** The provider ids the route was called with, in order. */
	providerModelsReads: string[];
	/** When set, the answer carries this warning, which is what a list that is not the upstream's reports. */
	providerModelsWarning: string | null;
	/**
	 * The custom node's own read, which the details card makes beside the provider read (§7.4). It is served
	 * here rather than by the node stub because a node's screen needs this route and the model routes
	 * together, and two stubs would fight over `fetch`: the node stub's own tests keep the writes.
	 */
	providerNode: StubModel | null;
	/**
	 * The settings document the Connections section reads for its rotation switch (§7.14). It is served here
	 * because the section renders on the same screen as the model routes, and a second stub would fight over
	 * `fetch`.
	 */
	settings: StubModel;
	/** The bodies `PATCH /settings` received, which is where the whole override map shows up. */
	settingsPatches: StubModel[];
	/** The settings write's own status, for a test that needs the gateway to refuse a rotation change. */
	settingsWriteStatus: number;
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

/** One entry of a combo test answer: a reference, what it resolved to, and how the probe went. */
export function comboProbeRow(overrides: StubModel = {}): StubModel {
	return {
		ref: 'openai/gpt-4o',
		role: 'model',
		ok: true,
		provider_id: 'openai',
		model_id: 'gpt-4o',
		endpoint_id: 'ep_01',
		latency_ms: 120,
		...overrides
	};
}

export function oauthEndpointRow(overrides: StubModel = {}): StubModel {
	return {
		endpoint_id: 'ep_oauth_1',
		label: 'xAI account',
		status: 'active',
		// Relative to the run, because the panel compares an expiry against its own clock: a fixed date would
		// read as fresh today and as expired in a year.
		expires_at: new Date(Date.now() + 36 * 60 * 60 * 1000).toISOString(),
		last_refresh_at: null,
		refresh_state: 'fresh',
		...overrides
	};
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
		combos: [],
		comboTestResults: [],
		comboTestStatus: 200,
		comboTests: [],
		hasOAuth: false,
		oauthFlow: 'code',
		oauthEndpoints: [],
		oauthReadStatus: 200,
		oauthStartRefusal: null,
		oauthRefreshRefusal: null,
		oauthStarts: [],
		oauthRefreshes: [],
		providers: ['openai', 'anthropic'],
		providerRows: [],
		disabledWrites: [],
		customCreates: [],
		customDeletes: [],
		reads: [],
		readStatus: 200,
		disabledReadStatus: 200,
		combosTotal: null,
		writeStatus: 200,
		authType: 'bearer',
		endpoints: [],
		endpointReadStatus: 200,
		endpointCreates: [],
		endpointBulkCreates: [],
		providerModels: [],
		providerModelsReadStatus: 200,
		providerModelsReads: [],
		providerModelsWarning: null,
		providerNode: null,
		settings: settingsDocument(),
		settingsPatches: [],
		settingsWriteStatus: 200,
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

		// The settings document, which the Connections section reads for its rotation switch and writes to
		// change one provider's entry. A write is applied to the stub's own document the way the server
		// applies it, so a read-modify-write test sees its change in the answer rather than in an echo.
		if (parsed.pathname.endsWith('/settings')) {
			if (method === 'PATCH') {
				stub.settingsPatches.push(body);

				if (stub.settingsWriteStatus !== 200) {
					return refusal(
						'VALIDATION_ERROR',
						'The gateway refused this value.',
						stub.settingsWriteStatus
					);
				}

				const routing = (body.routing ?? {}) as StubModel;
				stub.settings = {
					...stub.settings,
					routing: { ...(stub.settings.routing as StubModel), ...routing }
				};
			}

			return json(stub.settings);
		}

		// The node's own read, which the details card makes when the screen is a custom node's (§7.4). It is
		// matched before the provider read below because both end in a path segment.
		const nodeMatch = /\/provider-nodes\/([^/?]+)$/.exec(parsed.pathname);
		if (method === 'GET' && nodeMatch) {
			if (stub.providerNode === null) {
				return refusal('NOT_FOUND', 'That provider no longer exists.', 404);
			}
			return json(stub.providerNode);
		}

		// The page's own load, and the endpoint list its last section reads. Both answer the shape the
		// panel's schemas expect, so a test can render the whole screen and assert on one section.
		//
		// The provider's own model list is checked first, because its path is the provider path plus a
		// segment: a node's import reads it, and the answer is the rows plus, when the list is a fallback
		// rather than a fresh answer, the warning that says so.
		const providerModelsMatch = /\/providers\/([^/?]+)\/models$/.exec(parsed.pathname);
		if (method === 'GET' && providerModelsMatch) {
			stub.providerModelsReads.push(decodeURIComponent(providerModelsMatch[1]));

			if (stub.providerModelsReadStatus !== 200) {
				return refusal(
					'INTERNAL_ERROR',
					'The provider models could not be read.',
					stub.providerModelsReadStatus
				);
			}

			return json({
				data: stub.providerModels,
				...(stub.providerModelsWarning === null ? {} : { warning: stub.providerModelsWarning })
			});
		}

		// The provider list, which the picker reads for its active set and its group names. It is served
		// paginated the way the route is, so a test can hold the loader's page walk rather than a
		// one-page answer that would pass for a loader which never asked for a second page.
		if (method === 'GET' && parsed.pathname.endsWith('/providers')) {
			const page = Number(parsed.searchParams.get('page') ?? 1);
			const perPage = Number(parsed.searchParams.get('per_page') ?? 25);
			const start = (page - 1) * perPage;
			return json({
				data: stub.providerRows.slice(start, start + perPage),
				meta: { page, per_page: perPage, total: stub.providerRows.length }
			});
		}

		const providerMatch = /\/providers\/([^/?]+)$/.exec(parsed.pathname);
		if (method === 'GET' && providerMatch) {
			return json(
				providerDetailRow({
					id: decodeURIComponent(providerMatch[1]),
					auth_type: stub.authType,
					has_oauth: stub.hasOAuth
				})
			);
		}
		if (method === 'GET' && parsed.pathname.endsWith('/endpoints')) {
			if (stub.endpointReadStatus !== 200) {
				return refusal(
					'INTERNAL_ERROR',
					'The connections could not be read.',
					stub.endpointReadStatus
				);
			}
			return json({
				data: stub.endpoints,
				meta: { page: 1, per_page: 25, total: stub.endpoints.length }
			});
		}

		// The create route, which is what the page's key dialog posts to. It refuses an endpoint the API
		// would refuse (an `api_key` endpoint carries at least one key, `endpoint_create.go:53-55`) and
		// answers the row the list then renders, so the page's own re-read is visible in a test.
		if (method === 'POST' && parsed.pathname.endsWith('/endpoints')) {
			stub.endpointCreates.push(body);

			if (stub.writeStatus !== 200) {
				return refusal('INTERNAL_ERROR', 'The endpoint could not be stored.', stub.writeStatus);
			}

			const keys = Array.isArray(body.keys) ? (body.keys as StubModel[]) : [];
			if (keys.length === 0 && ['api_key', 'apikey'].includes(String(body.auth_type ?? ''))) {
				return refusal('VALIDATION_ERROR', 'field Keys failed validation: min', 400);
			}

			const row = endpointRow({
				id: `ep_${stub.endpoints.length + 1}`,
				provider_id: String(body.provider_id ?? ''),
				label: String(body.label ?? ''),
				auth_type: String(body.auth_type ?? ''),
				priority: typeof body.priority === 'number' ? body.priority : 1,
				key_count: keys.length,
				active_key_count: keys.length
			});
			stub.endpoints = [row, ...stub.endpoints];
			return json(row, 201);
		}

		// The batch create the provider screen's paste posts to: one element per connection, all-or-nothing
		// (§8.1). It refuses the two things the API refuses, a row with no key and a label already stored or
		// repeated inside the batch (the unique index at
		// `app-serv/migrations/000005_upstream_endpoints.up.sql:29`), and reports the offending row by
		// index, which is what the dialog re-keys onto the pasted line.
		if (method === 'POST' && parsed.pathname.endsWith('/endpoints/bulk')) {
			stub.endpointBulkCreates.push(body);

			if (stub.writeStatus !== 200) {
				return refusal('INTERNAL_ERROR', 'The connections could not be stored.', stub.writeStatus);
			}

			const rows = Array.isArray(body.endpoints) ? (body.endpoints as StubModel[]) : [];
			const seen = new Set(stub.endpoints.map((row) => String(row.label).toLowerCase()));

			for (let index = 0; index < rows.length; index += 1) {
				const item = rows[index];
				const keys = Array.isArray(item.keys) ? (item.keys as StubModel[]) : [];
				const label = String(item.label ?? '');
				const failure =
					keys.length === 0
						? 'field Keys failed validation: min'
						: seen.has(label.toLowerCase())
							? 'An endpoint with this label already exists for this provider.'
							: null;

				if (failure !== null) {
					return json(
						{
							error: { code: 'VALIDATION_ERROR', message: failure },
							results: rows.map((_, position) =>
								position === index ? { index: position, error: failure } : { index: position }
							)
						},
						400
					);
				}
				seen.add(label.toLowerCase());
			}

			const created = rows.map((item, index) => {
				const keys = Array.isArray(item.keys) ? (item.keys as StubModel[]) : [];
				return endpointRow({
					id: `ep_bulk_${stub.endpoints.length + index + 1}`,
					provider_id: String(body.provider_id ?? ''),
					label: String(item.label ?? ''),
					auth_type: String(body.auth_type ?? ''),
					priority: typeof item.priority === 'number' ? item.priority : 1,
					key_count: keys.length,
					active_key_count: keys.length
				});
			});
			stub.endpoints = [...created, ...stub.endpoints];
			return json({ created, results: created.map((row, index) => ({ index, id: row.id })) }, 201);
		}

		const oauthMatch = /\/providers\/([^/]+)\/oauth\/(status|start|refresh)$/.exec(parsed.pathname);

		if (method === 'GET' && oauthMatch?.[2] === 'status') {
			stub.reads.push('oauth:status');
			if (stub.oauthReadStatus !== 200) {
				return refusal(
					'INTERNAL_ERROR',
					'The oauth state could not be read.',
					stub.oauthReadStatus
				);
			}
			return json({
				// The provider the route names, so a test cannot pass against an answer about another one.
				provider_id: decodeURIComponent(oauthMatch[1]),
				flow: stub.oauthFlow,
				endpoints: stub.oauthEndpoints
			});
		}

		if (method === 'POST' && oauthMatch?.[2] === 'start') {
			stub.oauthStarts.push(decodeURIComponent(oauthMatch[1]));
			if (stub.oauthStartRefusal) {
				const { status, code, message } = stub.oauthStartRefusal;
				return refusal(code, message, status);
			}
			return json({ authorize_url: 'https://provider.test/authorize?state=st_1', state: 'st_1' });
		}

		if (method === 'POST' && oauthMatch?.[2] === 'refresh') {
			const endpointId = (body.endpoint_id as string | undefined) ?? null;
			stub.oauthRefreshes.push(endpointId);
			if (stub.oauthRefreshRefusal) {
				const { status, code, message } = stub.oauthRefreshRefusal;
				return refusal(code, message, status);
			}

			// A real refresh moves the token: the named account, or every due one, becomes fresh. The panel
			// re-reads the status afterwards, so a stub that answered without moving anything would let a
			// panel that never re-reads pass.
			const moved: string[] = [];
			stub.oauthEndpoints = stub.oauthEndpoints.map((endpoint) => {
				const due = endpoint.refresh_state === 'due';
				if (endpointId !== null ? endpoint.endpoint_id !== endpointId : !due) return endpoint;
				moved.push(String(endpoint.endpoint_id));
				return {
					...endpoint,
					refresh_state: 'fresh',
					last_refresh_at: '2026-09-20T12:00:00Z'
				};
			});

			return json({ refreshed: moved.length, endpoint_ids: moved });
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

		// The probe route answers from the stored combo, not from the request: the id it was asked for is
		// the combo whose name and strategy come back, so a panel that rendered its own row's values would
		// pass against a stub that echoed the body back.
		const comboTestMatch = /\/combos\/([^/?]+)\/test$/.exec(parsed.pathname);
		if (method === 'POST' && comboTestMatch) {
			const id = decodeURIComponent(comboTestMatch[1]);
			stub.comboTests.push(id);

			if (stub.comboTestStatus !== 200) {
				return refusal('NOT_FOUND', 'combo not found', stub.comboTestStatus);
			}

			const combo = stub.combos.find((entry) => entry.id === id);
			if (!combo) return refusal('NOT_FOUND', 'combo not found', 404);

			return json({
				combo_id: combo.id,
				combo: combo.name,
				strategy: combo.strategy,
				results: stub.comboTestResults
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

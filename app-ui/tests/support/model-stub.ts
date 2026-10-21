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
import { settingsDocument } from './settings-document';
import type { StubContext } from './model-stub-context';
import type { ModelStub, StubModel } from './model-stub-types';
import { handleProviderRoutes } from './model-stub-routes-providers';
import { handleEndpointRoutes } from './model-stub-routes-endpoints';
import { handleOAuthRoutes } from './model-stub-routes-oauth';
import { handleModelRoutes } from './model-stub-routes-models';

export type { ModelStub, StubModel } from './model-stub-types';
export {
	catalogRow,
	comboProbeRow,
	comboRow,
	customRow,
	oauthEndpointRow,
	providerDetailRow
} from './model-stub-fixtures';

export function stubModels(overrides: Partial<ModelStub> = {}): ModelStub {
	const stub: ModelStub = {
		catalog: [],
		disabled: [],
		custom: [],
		combos: [],
		comboTestResults: [],
		comboTestStatus: 200,
		comboTests: [],
		modelTests: [],
		modelSweeps: [],
		modelTestRows: [],
		modelTestTotal: null,
		modelTestStopped: null,
		modelTestStatus: 200,
		hasOAuth: false,
		oauthFlow: 'code',
		oauthEndpoints: [],
		oauthReadStatus: 200,
		oauthStartRefusal: null,
		oauthRefreshRefusal: null,
		oauthStarts: [],
		oauthDeviceStart: {
			device_code: 'dev_1',
			verification_url: 'https://vendor.test/device/selectAccounts?challenge=c1&nonce=dev_1',
			user_code: 'AB12CD34',
			interval_seconds: 0,
			expires_in: 300
		},
		oauthDeviceStartRefusal: null,
		oauthDevicePollAnswers: [],
		oauthDevicePollRefusalAt: 0,
		oauthDevicePollRefusal: null,
		oauthDeviceStarts: [],
		oauthDevicePolls: [],
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
		authModes: [],
		authHint: null,
		endpoints: [],
		endpointReadStatus: 200,
		endpointCreates: [],
		endpointDeletes: [],
		endpointDeleteStatus: 200,
		endpointBulkCreates: [],
		providerModels: [],
		providerModelsReadStatus: 200,
		providerModelsReads: [],
		providerModelsWarning: null,
		providerNode: null,
		settings: settingsDocument(),
		settingsPatches: [],
		settingsWriteStatus: 200,
		settingsReadStatus: 200,
		thinkingLevels: null,
		proxies: [],
		proxyReadStatus: 200,
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
		const body: StubModel = init?.body === undefined ? {} : JSON.parse(String(init.body));
		const parsed = new URL(String(input), 'http://panel.test');
		const context: StubContext = {
			stub,
			method,
			body,
			parsed,
			json,
			refusal,
			key,
			blocked,
			catalogWithCustom
		};

		return (
			handleProviderRoutes(context) ??
			handleEndpointRoutes(context) ??
			handleOAuthRoutes(context) ??
			handleModelRoutes(context)
		);
	});

	return stub;
}

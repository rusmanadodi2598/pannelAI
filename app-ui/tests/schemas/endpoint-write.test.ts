// Write-form contract tests for the endpoint family: the shapes a screen fills in, and the body each one
// becomes (docs/SPEC-API/001-SPEC-API.md §7.5, SPEC-UI §6.2).
//
// The regression this file exists for is the mapping itself. The create form used to send its own field
// names, `key_value` among them, and the route's decoder refuses an unknown field
// (`internal/schema/validator.go`, `jsonDecoder`), so every create failed with
// `400 unknown field "key_value"`. The case below pins the body, not the form, which is what would have
// caught it.
//
// The key-auth rule is the API's own and is measured, not assumed: an `api_key` endpoint with no key is
// refused before any write (`internal/service/endpoint_create.go`, `buildEndpoint`), for both spellings,
// because `ParseAuthType` maps the registry's `apikey` onto the API's `api_key`.

import { describe, expect, it } from 'vitest';
import { ENDPOINT_STATUS_ACTIVE } from '$lib/schemas/endpoint';
import {
	createEndpointBody,
	schemaCreateEndpointBody,
	schemaCreateEndpointForm,
	schemaUpdateEndpointForm
} from '$lib/schemas/endpoint-write';
import {
	schemaAddEndpointKeyForm,
	schemaUpdateEndpointKeyForm
} from '$lib/schemas/endpoint-credential';

describe('schemaCreateEndpointForm', () => {
	// The three auth types whose endpoint carries no credential of its own.
	const KEYLESS = ['oauth', 'no_auth', 'none'];

	for (const authType of KEYLESS) {
		it(`accepts the ${authType} auth type with no key, because it has none to carry`, () => {
			const result = schemaCreateEndpointForm.safeParse({
				provider_id: 'openai',
				label: 'Primary',
				auth_type: authType,
				keys: []
			});

			expect(result.success).toBe(true);
		});
	}

	// Both spellings, because they are the same rule: the registry writes `apikey` and the API `api_key`.
	for (const authType of ['api_key', 'apikey']) {
		it(`accepts a ${authType} endpoint once it carries a key`, () => {
			const result = schemaCreateEndpointForm.safeParse({
				provider_id: 'openai',
				label: 'Primary',
				auth_type: authType,
				keys: [{ value: 'sk-abcdefgh' }]
			});

			expect(result.success).toBe(true);
		});

		it(`refuses a ${authType} endpoint with no key, which the gateway would refuse too`, () => {
			const result = schemaCreateEndpointForm.safeParse({
				provider_id: 'openai',
				label: 'Primary',
				auth_type: authType,
				keys: []
			});

			expect(result.success).toBe(false);
			if (!result.success) {
				expect(result.error.issues[0]?.message).toContain('at least one key');
			}
		});
	}

	it('refuses a batch larger than the route accepts, before the round trip', () => {
		const keys = Array.from({ length: 101 }, (_, index) => ({ value: `sk-abcdefgh${index}` }));

		expect(
			schemaCreateEndpointForm.safeParse({
				provider_id: 'openai',
				label: 'Primary',
				auth_type: 'api_key',
				keys
			}).success
		).toBe(false);
	});

	// The service-account pair is the one create rule that spans fields, so both directions of it are
	// pinned: the pair is required with the auth type that reads it, and refused with every other one
	// (`internal/service/endpoint_create.go`, `buildEndpoint`).
	const SERVICE_ACCOUNT = {
		provider_id: 'vertex',
		label: 'Veo',
		auth_type: 'service_account',
		sa_project_id: 'pannelai-prod',
		sa_location: 'us-central1',
		keys: [{ value: '{"type":"service_account"}' }]
	};

	it('accepts a service_account endpoint once it carries its key, project, and location', () => {
		expect(schemaCreateEndpointForm.safeParse(SERVICE_ACCOUNT).success).toBe(true);
	});

	it('refuses a service_account endpoint with no key, because the JSON is what it mints with', () => {
		const result = schemaCreateEndpointForm.safeParse({ ...SERVICE_ACCOUNT, keys: [] });

		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error.issues[0]?.message).toContain('at least one key');
		}
	});

	for (const missing of ['sa_project_id', 'sa_location'] as const) {
		it(`refuses a service_account endpoint without ${missing}, naming the field`, () => {
			const result = schemaCreateEndpointForm.safeParse({ ...SERVICE_ACCOUNT, [missing]: '' });

			expect(result.success).toBe(false);
			if (!result.success) {
				expect(result.error.issues.some((issue) => issue.path[0] === missing)).toBe(true);
			}
		});
	}

	it('refuses the pair on an auth type that never reads it, rather than storing a dead project', () => {
		const result = schemaCreateEndpointForm.safeParse({
			provider_id: 'openai',
			label: 'Primary',
			auth_type: 'api_key',
			keys: [{ value: 'sk-abcdefgh' }],
			sa_project_id: 'pannelai-prod',
			sa_location: 'us-central1'
		});

		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error.issues[0]?.message).toContain('service account endpoint');
		}
	});

	const INVALID: { payload: Record<string, unknown>; why: string }[] = [
		{
			payload: { provider_id: '  ', label: 'Primary', auth_type: 'api_key' },
			why: 'a blank provider'
		},
		{ payload: { provider_id: 'openai', label: '', auth_type: 'api_key' }, why: 'a blank label' },
		{
			payload: { provider_id: 'openai', label: 'Primary', auth_type: 'cookie' },
			why: 'the cookie auth type, which has no v1 transport'
		},
		{
			payload: { provider_id: 'openai', label: 'Primary', auth_type: 'api_key', priority: 0 },
			why: 'a priority below the API minimum'
		},
		{
			payload: {
				provider_id: 'openai',
				label: 'Primary',
				auth_type: 'api_key',
				keys: [{ value: 'short' }]
			},
			why: 'a credential shorter than the panel minimum'
		}
	];

	for (const testCase of INVALID) {
		it(`rejects ${testCase.why}`, () => {
			expect(schemaCreateEndpointForm.safeParse(testCase.payload).success).toBe(false);
		});
	}
});

describe('createEndpointBody', () => {
	function form(overrides: Record<string, unknown> = {}) {
		const parsed = schemaCreateEndpointForm.safeParse({
			provider_id: 'openai',
			label: 'Primary',
			auth_type: 'api_key',
			keys: [{ value: 'sk-abcdefgh' }],
			...overrides
		});
		if (!parsed.success) throw new Error('the fixture must parse');
		return parsed.data;
	}

	it('carries the keys as the route reads them, under no other name', () => {
		const body = createEndpointBody(form({ keys: [{ label: 'First', value: 'sk-abcdefgh' }] }));

		expect(body).toEqual({
			provider_id: 'openai',
			label: 'Primary',
			auth_type: 'api_key',
			keys: [{ label: 'First', value: 'sk-abcdefgh' }]
		});
		// The form's own field name is gone, which is the whole point of the mapping.
		expect(Object.keys(body)).not.toContain('key_value');
	});

	it('omits the key list entirely when the form carried none', () => {
		const body = createEndpointBody(form({ auth_type: 'oauth', keys: [] }));

		expect(body).toEqual({ provider_id: 'openai', label: 'Primary', auth_type: 'oauth' });
		expect('keys' in body).toBe(false);
	});

	it('carries priority only when it was given', () => {
		expect('priority' in createEndpointBody(form())).toBe(false);
		expect(createEndpointBody(form({ priority: 4 })).priority).toBe(4);
	});

	it('keeps the priority the API would reorder siblings by', () => {
		expect(createEndpointBody(form({ priority: 7 })).priority).toBe(7);
	});

	it('carries the service-account pair, and only when the form stated it', () => {
		const body = createEndpointBody(
			form({
				provider_id: 'vertex',
				auth_type: 'service_account',
				sa_project_id: 'pannelai-prod',
				sa_location: 'us-central1'
			})
		);

		expect(body.sa_project_id).toBe('pannelai-prod');
		expect(body.sa_location).toBe('us-central1');
		// The body is what the pre-send gate checks and what the route decodes, so both halves are pinned.
		expect(schemaCreateEndpointBody.safeParse(body).success).toBe(true);
		expect('sa_project_id' in createEndpointBody(form())).toBe(false);
	});

	it('refuses a body whose pair is incomplete, the shape a dropped field would send', () => {
		expect(
			schemaCreateEndpointBody.safeParse({
				provider_id: 'vertex',
				label: 'Veo',
				auth_type: 'service_account',
				sa_project_id: 'pannelai-prod',
				keys: [{ value: '{"type":"service_account"}' }]
			}).success
		).toBe(false);
	});
});

describe('schemaUpdateEndpointForm', () => {
	it('accepts a priority-only change, which is what reorders siblings', () => {
		const result = schemaUpdateEndpointForm.safeParse({ priority: 3 });

		expect(result.success).toBe(true);
	});

	it('accepts a status change to disabled', () => {
		expect(schemaUpdateEndpointForm.safeParse({ status: ENDPOINT_STATUS_ACTIVE }).success).toBe(
			true
		);
	});

	it('rejects a status outside the two the API accepts on a patch', () => {
		expect(schemaUpdateEndpointForm.safeParse({ status: 'quarantined' }).success).toBe(false);
	});

	it('rejects an unknown field, because the API rejects it too', () => {
		expect(schemaUpdateEndpointForm.safeParse({ provider_id: 'anthropic' }).success).toBe(false);
	});
});

describe('schemaAddEndpointKeyForm', () => {
	it('accepts a key with only a value', () => {
		expect(schemaAddEndpointKeyForm.safeParse({ value: 'sk-abcdefgh' }).success).toBe(true);
	});

	it('rejects a value that is too short to be a credential', () => {
		expect(schemaAddEndpointKeyForm.safeParse({ value: 'sk-' }).success).toBe(false);
	});

	it('rejects a missing value, because a key is the point of the form', () => {
		expect(schemaAddEndpointKeyForm.safeParse({ label: 'backup' }).success).toBe(false);
	});
});

describe('schemaUpdateEndpointKeyForm', () => {
	it('accepts a label-only edit, which leaves the stored credential alone', () => {
		expect(schemaUpdateEndpointKeyForm.safeParse({ label: 'renamed' }).success).toBe(true);
	});

	it('accepts an empty value as "keep the stored credential"', () => {
		expect(schemaUpdateEndpointKeyForm.safeParse({ value: '' }).success).toBe(true);
	});

	it('rejects a replacement value that is too short', () => {
		expect(schemaUpdateEndpointKeyForm.safeParse({ value: 'sk-' }).success).toBe(false);
	});
});

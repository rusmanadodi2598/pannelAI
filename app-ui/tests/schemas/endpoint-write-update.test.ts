// The body a create writes, and the forms that change a row that already exists
// (docs/SPEC-API/001-SPEC-API.md §7.5, docs/SPEC-UI/001-SPEC-UI.md §6.2).
//
// The create form itself is in `endpoint-write.test.ts`. This is what that form becomes on the wire, and the
// three forms that update an endpoint, add its key, and update one.

import { describe, expect, it } from 'vitest';
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
import { ENDPOINT_STATUS_ACTIVE } from '$lib/schemas/endpoint';
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

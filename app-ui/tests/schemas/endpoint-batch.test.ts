// Contract tests for the two batch shapes the endpoint family sends (docs/SPEC-API/001-SPEC-API.md §7.5,
// §8.1). They live apart from the one-at-a-time forms because they are the shapes a *paste* becomes, and
// because the file holding both crosses the panel's line limit.
//
// The second batch is the one the concept rests on: `POST /endpoints/bulk` states the provider and the
// auth type once and then carries one element per connection, which is how three pasted keys become three
// connections of the provider rather than one connection holding three keys.

import { describe, expect, it } from 'vitest';
import {
	bulkCreateEndpointsBody,
	schemaBulkAddKeysForm,
	schemaBulkCreateEndpointsBody
} from '$lib/schemas/endpoint-write';

describe('schemaBulkAddKeysForm', () => {
	it('accepts the API batch maximum', () => {
		const keys = Array.from({ length: 100 }, (_, index) => ({ value: `sk-abcdefgh${index}` }));

		expect(schemaBulkAddKeysForm.safeParse({ keys }).success).toBe(true);
	});

	it('rejects an empty batch', () => {
		expect(schemaBulkAddKeysForm.safeParse({ keys: [] }).success).toBe(false);
	});

	it('rejects a batch over the API maximum rather than letting the server refuse it', () => {
		const keys = Array.from({ length: 101 }, (_, index) => ({ value: `sk-abcdefgh${index}` }));

		expect(schemaBulkAddKeysForm.safeParse({ keys }).success).toBe(false);
	});

	it('rejects a batch where one row is invalid, naming no row but refusing the whole batch', () => {
		const result = schemaBulkAddKeysForm.safeParse({
			keys: [{ value: 'sk-abcdefgh' }, { value: 'bad' }]
		});

		expect(result.success).toBe(false);
	});
});

describe('the batch of connections', () => {
	// The provider and auth type are stated once, and every row carries its own name.
	const batch = { provider_id: 'openai', auth_type: 'api_key' };

	it('states the provider and auth type once, then one element per key', () => {
		const body = bulkCreateEndpointsBody('openai', 'api_key', [
			{ label: 'production', value: 'sk-abcdefgh' },
			{ label: 'Key 1', value: 'sk-ijklmnop' }
		]);

		expect(body).toEqual({
			...batch,
			endpoints: [
				{ label: 'production', keys: [{ value: 'sk-abcdefgh' }] },
				{ label: 'Key 1', keys: [{ value: 'sk-ijklmnop' }] }
			]
		});
		expect(schemaBulkCreateEndpointsBody.safeParse(body).success).toBe(true);
	});

	it('rejects an empty batch and one past the API cap, in the panel rather than after the round trip', () => {
		const overCap = Array.from({ length: 51 }, (_, index) => ({
			label: `Key ${index}`,
			keys: [{ value: `sk-abcdefgh${index}` }]
		}));

		expect(schemaBulkCreateEndpointsBody.safeParse({ ...batch, endpoints: [] }).success).toBe(
			false
		);
		expect(schemaBulkCreateEndpointsBody.safeParse({ ...batch, endpoints: overCap }).success).toBe(
			false
		);
	});

	it('refuses a form-only field, so the route never has to refuse one', () => {
		expect(
			schemaBulkCreateEndpointsBody.safeParse({
				...batch,
				endpoints: [{ label: 'Key 1', value: 'sk-abcdefgh' }]
			}).success
		).toBe(false);
	});

	it('refuses a row with no key, which the API refuses for the key auth types', () => {
		expect(
			schemaBulkCreateEndpointsBody.safeParse({
				...batch,
				endpoints: [{ label: 'Key 1', keys: [] }]
			}).success
		).toBe(false);
	});
});

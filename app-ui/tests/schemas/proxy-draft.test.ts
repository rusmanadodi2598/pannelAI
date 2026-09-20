// Proxy form draft-helper tests (docs/SPEC-UI/001-SPEC-UI.md §6.9).
//
// The write-only password rule lives here as much as in the schema: a dialog that prefilled the field
// from a stored row would be putting a secret back into the page the API deliberately never sends,
// and one that cleared a stored secret on an empty save would destroy a credential. The helpers decide
// what the dialog opens with, so they are what these cases pin down.
//
// The form schema itself is `proxy-form.test.ts`; this file is the draft and the help text.

import { describe, expect, it } from 'vitest';
import type { Proxy } from '$lib/schemas/proxy';
import {
	emptyProxyDraft,
	proxyCandidateFromDraft,
	proxyDraftFrom,
	proxyPasswordHelp
} from '$lib/schemas/proxy-form';
import { forEachCase } from '../support/tables';

function storedProxy(overrides: Partial<Proxy> = {}): Proxy {
	return {
		id: 'prx_01HZZ9K2',
		label: 'Frankfurt egress',
		protocol: 'https',
		host: 'proxy.example.com',
		port: 8443,
		username: 'operator',
		has_password: true,
		enabled: true,
		created_at: '2026-09-19T09:00:00Z',
		updated_at: '2026-09-19T09:05:00Z',
		...overrides
	};
}

describe('emptyProxyDraft', () => {
	it('starts an add with an empty port and the API default for enabled', () => {
		expect(emptyProxyDraft()).toEqual({
			label: '',
			protocol: 'http',
			host: '',
			port: '',
			username: '',
			password: '',
			enabled: true
		});
	});
});

describe('proxyDraftFrom', () => {
	it('opens a shut dialog the same way, so nothing is left behind for the next one', () => {
		expect(proxyDraftFrom(null)).toEqual(emptyProxyDraft());
	});

	it('treats the add marker as an empty draft', () => {
		expect(proxyDraftFrom('new')).toEqual(emptyProxyDraft());
	});

	it('copies a stored row without its password, which the API never sends', () => {
		expect(proxyDraftFrom(storedProxy())).toEqual({
			label: 'Frankfurt egress',
			protocol: 'https',
			host: 'proxy.example.com',
			port: '8443',
			username: 'operator',
			password: '',
			enabled: true
		});
	});

	it('carries a disabled row through as disabled', () => {
		expect(proxyDraftFrom(storedProxy({ enabled: false })).enabled).toBe(false);
	});

	it('renders the port as text, because the field is text and the schema coerces it', () => {
		expect(proxyDraftFrom(storedProxy({ port: 1080 })).port).toBe('1080');
	});
});

describe('proxyCandidateFromDraft', () => {
	it('takes the address and the credentials and leaves the label out', () => {
		const candidate = proxyCandidateFromDraft(
			proxyDraftFrom(storedProxy({ username: 'operator', has_password: false }))
		);

		expect(candidate).toEqual({
			protocol: 'https',
			host: 'proxy.example.com',
			port: '8443',
			username: 'operator',
			password: ''
		});
		expect('label' in candidate).toBe(false);
	});

	it('passes a typed password through to the test route', () => {
		const draft = { ...emptyProxyDraft(), password: 'hunter2' };

		expect(proxyCandidateFromDraft(draft).password).toBe('hunter2');
	});
});

describe('proxyPasswordHelp', () => {
	forEachCase(
		[
			{
				name: 'tells an add that a password is optional',
				target: 'new' as const,
				expected: 'Optional. An open proxy needs none.'
			},
			{
				name: 'tells an edit with a stored secret that empty keeps it',
				target: storedProxy({ has_password: true }),
				expected:
					'A password is stored. Leave this empty to keep it, or type a new one to replace it.'
			},
			{
				name: 'says so when no secret is stored',
				target: storedProxy({ has_password: false }),
				expected: 'No password is stored.'
			}
		],
		({ target, expected }) => {
			expect(proxyPasswordHelp(target)).toBe(expected);
		}
	);
});

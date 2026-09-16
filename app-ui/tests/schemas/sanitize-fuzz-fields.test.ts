// Randomised input tests for the field-shaped transforms in src/lib/schemas/sanitize.ts.
//
// Split from tests/schemas/sanitize-fuzz.test.ts so each file stays inside the 250 line limit the
// project enforces. The generator is shared through tests/support/corpus.ts, so both files assert
// against the same corpus and the same seed.

import { describe, expect, it } from 'vitest';
import {
	normalizeHost,
	normalizeNoProxyList,
	stripKeyWhitespace,
	stripTrailingSlash
} from '$lib/schemas/sanitize';
import { JUNK, PROFILES, corpus, where } from '../support/corpus';

describe('normalizeHost contract', () => {
	for (const profile of PROFILES) {
		it(`produces a trimmed lowercase host on ${profile.name}`, () => {
			for (const [index, input] of corpus(profile).entries()) {
				const at = where('normalizeHost', profile.name, index, input);
				const output = normalizeHost(input);

				expect(JUNK.test(output), at).toBe(false);
				expect(output, at).toBe(output.trim());
				expect(output, at).toBe(output.toLowerCase());
			}
		});
	}
});

describe('stripKeyWhitespace contract', () => {
	for (const profile of PROFILES) {
		it(`removes every whitespace character on ${profile.name}`, () => {
			for (const [index, input] of corpus(profile).entries()) {
				const at = where('stripKeyWhitespace', profile.name, index, input);
				const output = stripKeyWhitespace(input);

				expect(/\s/.test(output), at).toBe(false);
				expect(output, at).toBe(input.replace(/\s+/g, ''));
			}
		});
	}
});

describe('normalizeNoProxyList contract', () => {
	for (const profile of PROFILES) {
		it(`returns unique clean hosts on ${profile.name}`, () => {
			for (const [index, input] of corpus(profile).entries()) {
				const at = where('normalizeNoProxyList', profile.name, index, input);
				const hosts = normalizeNoProxyList(input);

				for (const host of hosts) {
					expect(host.length, at).toBeGreaterThan(0);
					expect(host, at).toBe(host.trim());
					expect(host, at).toBe(host.toLowerCase());
					// Only the comma is forbidden here. Internal whitespace survives, because SPEC-UI §7.2
					// gives `outbound_no_proxy` its own row: trimmed, deduplicated, empty entries dropped.
					// It does not require each entry to pass the proxy host rule, and that gap is recorded
					// in anti-slop/audit-004-2026-09-16.md rather than asserted away here.
					expect(host.includes(','), at).toBe(false);
				}

				expect(new Set(hosts).size, at).toBe(hosts.length);
				expect(hosts.length, at).toBeLessThanOrEqual(input.split(',').length);
				expect(normalizeNoProxyList(hosts.join(',')), at).toEqual(hosts);
			}
		});
	}
});

describe('stripTrailingSlash contract', () => {
	for (const profile of PROFILES) {
		it(`removes at most one slash on ${profile.name}`, () => {
			for (const [index, input] of corpus(profile).entries()) {
				const at = where('stripTrailingSlash', profile.name, index, input);
				const output = stripTrailingSlash(input);

				if (input.endsWith('/')) {
					expect(output, at).toBe(input.slice(0, -1));
				} else {
					expect(output, at).toBe(input);
				}

				let fullyStripped = input;
				while (fullyStripped.endsWith('/')) fullyStripped = stripTrailingSlash(fullyStripped);

				expect(fullyStripped.endsWith('/'), at).toBe(false);
			}
		});
	}
});

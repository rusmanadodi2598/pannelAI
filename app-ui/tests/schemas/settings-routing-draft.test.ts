// The Routing tab's two draft mappings (docs/SPEC-API/001-SPEC-API.md §7.14).
//
// Both decide what the tab is allowed to see, and both fail quietly if they are wrong: a key the tab does
// not own reaches a strict form that refuses it, and a shared nested object means editing the draft moves
// the loaded copy and the screen stops reporting an unsaved change at all.

import { describe, expect, it } from 'vitest';
import { routingDraftFromSettings, cloneRoutingDraft } from '$lib/schemas/settings-routing-draft';
import type { RoutingSettings } from '$lib/schemas/settings';

function group(overrides: Partial<RoutingSettings> = {}): RoutingSettings {
	return {
		combo_strategy: 'fallback',
		combo_sticky_limit: 1,
		sticky_limit: 3,
		fallback_strategy: 'fill-first',
		provider_strategies: { openai: { sticky_limit: 5 } },
		...overrides
	};
}

describe('routingDraftFromSettings', () => {
	it('leaves the provider override map out of the draft the tab owns', () => {
		expect('provider_strategies' in routingDraftFromSettings(group())).toBe(false);
	});

	it('holds a zero or absent park window blank, because that is the gateway default', () => {
		const draft = routingDraftFromSettings(
			group({ key_park: { auth_seconds: 0, rate_limit_seconds: 60 } })
		);

		expect(draft.key_park.auth_seconds).toBeUndefined();
		expect(draft.key_park.rate_limit_seconds).toBe(60);
	});

	it('names the stored failover budget, and the documented one when the group carries none', () => {
		expect(
			routingDraftFromSettings(group({ failover: { total_ms: 5000, max_attempts: 3 } })).failover
		).toEqual({ total_ms: 5000, max_attempts: 3 });

		expect(routingDraftFromSettings(group()).failover).toEqual({
			total_ms: 30000,
			max_attempts: 8
		});
	});
});

describe('cloneRoutingDraft', () => {
	it('copies the nested groups rather than sharing them', () => {
		const source = routingDraftFromSettings(group({ key_park: { auth_seconds: 120 } }));
		const copy = cloneRoutingDraft(source);

		copy.key_park.auth_seconds = 300;
		if (copy.failure_classes) copy.failure_classes.AUTH_INVALID = { action: 'park' };

		expect(source.key_park.auth_seconds).toBe(120);
		expect(source.failure_classes).toEqual({});
	});
});

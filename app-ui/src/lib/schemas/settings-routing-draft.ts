// The routing group as the Routing tab holds it: the loaded group narrowed to a draft, and a draft
// copied without sharing its nested objects (docs/SPEC-API/001-SPEC-API.md §7.14).
import { FAILOVER_DEFAULT_MAX_ATTEMPTS, FAILOVER_DEFAULT_TOTAL_MS } from './settings';
import { failureClassesToForm, namedFailureClasses } from './settings-failure-classes';
import type { RoutingSettings, RoutingSettingsForm } from './settings';

const parkSeconds = (value: number | undefined): number | undefined =>
	value && value > 0 ? value : undefined;

// The tab owns the group's global keys and nothing else. `provider_strategies` belongs to the provider
// screen's own switch, and carrying it here would put it in a strict form that refuses it and in a
// PATCH body the tab has no business writing. `key_park` windows are narrowed to what is actually
// stored: a zero or absent window is the gateway default, not a value the operator chose, so it is held
// blank. The class table follows the same rule from the other side, because the response answers only
// the classes an operator stored and a class it does not name is still running its documented default.
export function routingDraftFromSettings(group: RoutingSettings): RoutingSettingsForm {
	return {
		combo_strategy: group.combo_strategy,
		combo_sticky_limit: group.combo_sticky_limit,
		sticky_limit: group.sticky_limit,
		fallback_strategy: group.fallback_strategy,
		key_park: {
			auth_seconds: parkSeconds(group.key_park?.auth_seconds),
			rate_limit_seconds: parkSeconds(group.key_park?.rate_limit_seconds),
			transient_seconds: parkSeconds(group.key_park?.transient_seconds),
			billing_seconds: parkSeconds(group.key_park?.billing_seconds)
		},
		failover: {
			total_ms: group.failover?.total_ms ?? FAILOVER_DEFAULT_TOTAL_MS,
			max_attempts: group.failover?.max_attempts ?? FAILOVER_DEFAULT_MAX_ATTEMPTS
		},
		failure_classes: failureClassesToForm(group.failure_classes)
	};
}

// The two copies the tab keeps must be independent objects: `key_park`, `failover` and each class entry
// are nested, so a shallow copy would hand the draft and the loaded value the same sub-object, and
// editing one would silently edit the other.
export function cloneRoutingDraft(form: RoutingSettingsForm): RoutingSettingsForm {
	return {
		combo_strategy: form.combo_strategy,
		combo_sticky_limit: form.combo_sticky_limit,
		sticky_limit: form.sticky_limit,
		fallback_strategy: form.fallback_strategy,
		key_park: { ...form.key_park },
		failover: { ...form.failover },
		failure_classes: namedFailureClasses(form.failure_classes)
	};
}

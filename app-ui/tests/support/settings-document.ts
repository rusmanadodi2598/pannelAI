// The settings document fixture (docs/SPEC-API/001-SPEC-API.md §7.14).
//
// Three screens read this document: Settings, which edits four of its groups (§6.13), Proxy Pools,
// which edits the network group (§6.9), and a provider's detail screen, whose rotation switch and
// proxy card each own one key of it. One definition, so a change to the API's shape fails all three
// test files at once instead of one of them quietly.

export function settingsDocument(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		security: { require_login: true, require_api_key: true },
		routing: {
			combo_strategy: 'fallback',
			combo_sticky_limit: 1,
			sticky_limit: 3,
			fallback_strategy: 'fill-first',
			provider_strategies: {}
		},
		network: {
			outbound_proxy_enabled: false,
			outbound_proxy_url: '',
			outbound_no_proxy: '',
			outbound_proxy_strategy: 'fallback',
			provider_proxies: {}
		},
		logging: {
			request_capture_enabled: false,
			retention_days: 7,
			capture_body_max_bytes: 65536,
			observability_max_records: 1000
		},
		...overrides
	};
}

/**
 * The network group as the API answers it: the four keys the outbound form owns plus the
 * per-provider binding map (docs/PORT/009-PORT-PROVIDER-PROXY.md D1).
 *
 * A test that stores a proxy state uses this rather than a four-key literal, because the response
 * schema requires `provider_proxies`: a group without it is drift the read schema refuses, not a
 * stored document the screen would ever see.
 */
export function networkGroup(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	const network = settingsDocument().network as Record<string, unknown>;
	return { ...network, ...overrides };
}

/**
 * The routing group as the Settings Routing tab's form carries it: the four keys that tab owns.
 *
 * `provider_strategies` belongs to the provider screen's switch, and the form is strict, so a fixture
 * that handed the tab the whole routing group would fail the tab's own validation rather than test it.
 */
export function routingFormDocument(
	overrides: Record<string, unknown> = {}
): Record<string, unknown> {
	const routing = settingsDocument().routing as Record<string, unknown>;
	return {
		combo_strategy: routing.combo_strategy,
		combo_sticky_limit: routing.combo_sticky_limit,
		sticky_limit: routing.sticky_limit,
		fallback_strategy: routing.fallback_strategy,
		...overrides
	};
}

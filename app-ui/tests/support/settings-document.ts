// The settings document fixture (docs/SPEC-API/001-SPEC-API.md §7.14).
//
// Two screens read this document: Settings, which edits four of its groups (§6.13), and Proxy Pools,
// which edits the network group (§6.9). One definition, so a change to the API's shape fails both
// test files at once instead of one of them quietly.

export function settingsDocument(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		security: { require_login: true, require_api_key: true },
		routing: { combo_strategy: 'fallback', combo_sticky_limit: 1, sticky_limit: 3 },
		network: { outbound_proxy_enabled: false, outbound_proxy_url: '', outbound_no_proxy: '' },
		logging: {
			request_capture_enabled: false,
			retention_days: 7,
			capture_body_max_bytes: 65536,
			observability_max_records: 1000
		},
		...overrides
	};
}

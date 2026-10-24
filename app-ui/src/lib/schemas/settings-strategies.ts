// The closed sets a settings choice may take (docs/SPEC-API/001-SPEC-API.md §7.5, §7.14).
//
// The words are the API's own vocabulary: a form that accepted a fourth one would send a value the
// gateway refuses by name, and a label list that invented one would offer a mode nobody can route.

export const COMBO_STRATEGIES = ['fallback', 'round_robin', 'fusion'] as const;

export const COMBO_STRATEGY_LABELS: Record<(typeof COMBO_STRATEGIES)[number], string> = {
	fallback: 'Fallback',
	round_robin: 'Round robin',
	fusion: 'Fusion'
};

// The credential rotation modes. `fill-first` is the default and the value a document that predates the
// keys resolves to, so the read schema never answers an unset mode.
export const CREDENTIAL_ROTATIONS = ['fill-first', 'round-robin'] as const;

export const CREDENTIAL_ROTATION_LABELS: Record<(typeof CREDENTIAL_ROTATIONS)[number], string> = {
	'fill-first': 'Fill first',
	'round-robin': 'Round robin'
};

export type CredentialRotation = (typeof CREDENTIAL_ROTATIONS)[number];

// The proxy pool's rotation strategy (docs/PORT/008-PORT-PROXY-ENGINE.md D2). The API answers the
// effective value, never a blank (a document stored before the key reads as fallback), so the read
// schema takes the same closed set the write does.
export const PROXY_STRATEGIES = ['fallback', 'round_robin'] as const;

export const PROXY_STRATEGY_LABELS: Record<(typeof PROXY_STRATEGIES)[number], string> = {
	fallback: 'Fallback',
	round_robin: 'Round robin'
};

export type ProviderProxyStrategy = (typeof PROXY_STRATEGIES)[number];

// The provider-scoped overrides inside the settings document (docs/SPEC-API/001-SPEC-API.md §7.14).
//
// Each is one provider's entry in a map the gateway holds whole, so a screen that edits a row
// read-modify-writes the map and deletes an entry to return that provider to the global setting. The
// patches are separate from the tabs' own group patches for that reason: a tab writes the keys it
// rendered and nothing else.
import { z } from 'zod';
import { CREDENTIAL_ROTATIONS, PROXY_STRATEGIES } from './settings-strategies';

// One provider's override of the global rotation policy. Both fields are optional because an absent
// one inherits the global value: the panel renders what is stored rather than a filled-in copy, so a
// provider whose entry sets only the sticky limit does not read as if it had chosen a strategy.
export const schemaProviderStrategy = z.object({
	fallback_strategy: z.enum(CREDENTIAL_ROTATIONS).optional(),
	sticky_limit: z.number().int().min(1).optional()
});

// One provider's proxy binding (docs/PORT/009-PORT-PROVIDER-PROXY.md D1-D3). `__none__` is the
// reference's own sentinel for "None (direct)" (`NoAuthProxyCard.js`), and it is a real mode here:
// the provider dials direct even while the global switch is on. An absent `pool_id` is what Global
// looks like on the wire, not `''`, because an entry that sets neither field is the one shape the
// gateway refuses by name.
export const PROXY_POOL_NONE = '__none__';

// Both fields are optional because an absent one inherits the global setting: the panel renders what
// is stored rather than a filled-in copy, so an entry that only pins a pool does not read as if it
// had also chosen a strategy.
export const schemaProviderProxy = z.object({
	pool_id: z.string().optional(),
	strategy: z.enum(PROXY_STRATEGIES).optional()
});

export const schemaProviderProxyPatch = z.strictObject({
	pool_id: z.string().max(64, { message: 'Use 64 characters or fewer.' }).optional(),
	strategy: z.enum(PROXY_STRATEGIES).optional()
});

// The provider screen's own write: one provider's entry inside the whole override map (SPEC-API
// §7.14). It is a separate patch shape from the routing tab's because the two own different keys: the
// tab writes the global defaults and must not carry an override map it never rendered, and this one
// writes the map and must not carry the tab's fields. The map is sent whole, so a caller
// read-modify-writes it, and deleting an entry is how a provider returns to the global default.
export const schemaProviderStrategiesPatch = z.strictObject({
	routing: z.strictObject({
		provider_strategies: z.record(z.string(), schemaProviderStrategy)
	})
});

// The provider screen's proxy card: one provider's entry inside the whole binding map (SPEC-API
// §7.14). Separate from the network tab's patch for the same reason the strategies patch above is
// separate from the routing tab's: the tab writes the four outbound keys and must not carry a map it
// never rendered, and this one writes the map and must not carry the tab's fields. The map is sent
// whole, so a caller read-modify-writes it, and deleting an entry is how a provider returns to the
// global proxy setting.
export const schemaProviderProxiesPatch = z.strictObject({
	network: z.strictObject({
		provider_proxies: z.record(z.string(), schemaProviderProxyPatch)
	})
});

export type ProviderStrategy = z.infer<typeof schemaProviderStrategy>;
export type ProviderProxy = z.infer<typeof schemaProviderProxy>;
export type ProviderProxyPatch = z.infer<typeof schemaProviderProxyPatch>;
export type ProviderProxiesPatch = z.infer<typeof schemaProviderProxiesPatch>;
export type ProviderStrategiesPatch = z.infer<typeof schemaProviderStrategiesPatch>;

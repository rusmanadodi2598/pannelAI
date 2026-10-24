// What the gateway will do with one provider's proxy binding, in the sentence the card prints
// (docs/PORT/009-PORT-PROVIDER-PROXY.md D4). A derivation rather than markup, because the ordering it
// states is the engine's: a pinned pool leads and the others follow, so a pin never disables failover.

import { PROXY_POOL_NONE } from '$lib/schemas/settings-provider-overrides';
import {
	PROXY_STRATEGY_LABELS,
	type ProviderProxyStrategy
} from '$lib/schemas/settings-strategies';

export type ProxyBinding = {
	/** The select's pool: '' for the global setting, the sentinel for direct, or a pool id. */
	poolChoice: string;
	/** The provider's own strategy, or '' while it follows the global one. */
	strategyChoice: '' | ProviderProxyStrategy;
	/** The strategy the engine walks: the provider's own when it set one, the global one otherwise. */
	effectiveStrategy: ProviderProxyStrategy;
	/** The pinned pool's name, or null when the stored id names a row the pool no longer carries. */
	pinnedLabel: string | null;
	globalEnabled: boolean;
};

export function describeProxyBinding(binding: ProxyBinding): string {
	const { poolChoice, strategyChoice, effectiveStrategy, pinnedLabel, globalEnabled } = binding;
	const strategy = PROXY_STRATEGY_LABELS[effectiveStrategy];

	if (poolChoice === PROXY_POOL_NONE) {
		return 'Dials direct. The global proxy setting is not used.';
	}
	if (poolChoice !== '') {
		return pinnedLabel === null
			? `Pinned to ${poolChoice} (missing). Other pools carry this with ${strategy}.`
			: `Pinned to ${pinnedLabel}, which leads; other pools follow (${strategy}).`;
	}
	if (strategyChoice !== '') {
		return `Global pool, own strategy (${strategy}); proxying ${globalEnabled ? 'on' : 'off'}.`;
	}
	return `Follows the global proxy setting (${globalEnabled ? `on, ${strategy}` : 'off'}).`;
}

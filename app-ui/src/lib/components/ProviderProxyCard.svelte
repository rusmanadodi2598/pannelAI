<script lang="ts">
	// The provider screen's proxy binding card (docs/PORT/009-PORT-PROVIDER-PROXY.md D1-D5, D9;
	// docs/SPEC-UI/001-SPEC-UI.md §6.3, docs/SPEC-API/001-SPEC-API.md §7.14).
	//
	// This is the reference's per-provider control (`NoAuthProxyCard.js`) in our settings-map shape: it
	// writes one provider's entry of `network.provider_proxies` rather than a provider field, so it
	// reads the settings document for it. The map is one value (§7.14): a write sends the whole map
	// back, and a provider with no entry follows the global outbound setting.
	//
	// The pin rule is the owner's (D4), and it is why the copy does not repeat the reference's "pool
	// selector is ignored when rotation is active": here the pinned pool is the FIRST candidate and the
	// other usable pools follow with the strategy, so a pin never disables multi-pool failover. The
	// reference disables its pool select once a rotation strategy is on; this card keeps both editable
	// and states what the engine will do instead.
	//
	// It is its own component because it is a self-contained control: its own reads (the settings
	// document and the pool), its own write, and the lines that report them. The page only decides
	// where it sits.
	import { untrack } from 'svelte';
	import { listProxies } from '$lib/api/proxies';
	import { fetchSettings, patchProviderProxies } from '$lib/api/settings';
	import type { Proxy } from '$lib/schemas/proxy';
	import {
		PROXY_POOL_NONE,
		PROXY_STRATEGIES,
		PROXY_STRATEGY_LABELS,
		type PanelSettings,
		type ProviderProxy,
		type ProviderProxyPatch,
		type ProviderProxyStrategy
	} from '$lib/schemas/settings';

	let { providerId }: { providerId: string } = $props();

	// The binding map as last read, kept whole so a write returns it whole.
	let entries = $state<Record<string, ProviderProxy>>({});
	let globalEnabled = $state(false);
	let globalStrategy = $state<ProviderProxyStrategy>('fallback');
	let pools = $state<Proxy[]>([]);
	// The controls' own state rather than a derivation of the map: the browser moves a select before the
	// write is answered, and a refused write must put it back instead of leaving a control that claims a
	// change the gateway never stored.
	let poolChoice = $state('');
	let strategyChoice = $state<'' | ProviderProxyStrategy>('');
	let loading = $state(true);
	let error = $state<string | null>(null);
	/** A pool list that could not be read. The binding is still usable, so this does not blank the card. */
	let poolNotice = $state<string | null>(null);
	let saving = $state(false);
	let message = $state<string | null>(null);

	// The strategy this provider would walk with: its own when it set one, the global one otherwise,
	// because that is the order the engine would run.
	const effectiveStrategy = $derived(strategyChoice === '' ? globalStrategy : strategyChoice);
	// The pinned row's own name, or null when the stored id names a row the pool no longer carries.
	const pinnedLabel = $derived(pools.find((pool) => pool.id === poolChoice)?.label ?? null);

	// The rows the select offers, with the stored pin kept as an option even when its row is gone: a
	// select that silently fell back to Global would drop the stored pin on the next strategy change,
	// and a pinned row can be deleted from the pool screen after the pin was stored.
	const poolOptions = $derived.by(() => {
		const options = [
			{ value: '', label: 'Global' },
			{ value: PROXY_POOL_NONE, label: 'None (direct)' },
			...pools.map((pool) => ({ value: pool.id, label: pool.label }))
		];
		const known = poolChoice === '' || poolChoice === PROXY_POOL_NONE || pinnedLabel !== null;
		if (!known) options.push({ value: poolChoice, label: `${poolChoice} (missing)` });
		return options;
	});

	// What the engine will actually do with this provider, stated rather than left to be worked out. The
	// name avoids `state`: in a Svelte component `$state` is the rune, and a plain `state` binding makes
	// every rune in the file resolve as a subscription of it.
	const summary = $derived(describeSummary());

	$effect(() => {
		// The id is read here so a route swap re-reads the binding, and handed on so a slower earlier
		// read cannot overwrite a newer one.
		const id = providerId;
		untrack(() => void load(id));
	});

	async function load(id: string): Promise<void> {
		loading = true;
		const [settings, list] = await Promise.all([fetchSettings(), listProxies()]);
		loading = false;
		if (id !== providerId) return;

		if (!settings.ok) {
			error = settings.error.message;
			return;
		}

		error = null;
		poolNotice = list.ok ? null : list.error.message;
		pools = list.ok ? list.data.data : [];
		applyNetwork(settings.data.network);
	}

	/** Takes the network group the server stored, which is what both a read and a write answer with. */
	function applyNetwork(network: PanelSettings['network']): void {
		entries = { ...network.provider_proxies };
		globalEnabled = network.outbound_proxy_enabled;
		globalStrategy = network.outbound_proxy_strategy;
		syncControls();
	}

	/** Points both selects at what is stored for this provider. */
	function syncControls(): void {
		const entry = entries[providerId] ?? null;
		poolChoice = entry?.pool_id ?? '';
		strategyChoice = entry?.strategy ?? '';
	}

	function describeSummary(): string {
		if (poolChoice === PROXY_POOL_NONE) {
			return 'Dials direct; the global proxy setting is not used for this provider.';
		}
		if (poolChoice !== '') {
			const strategy = PROXY_STRATEGY_LABELS[effectiveStrategy];
			if (pinnedLabel === null) {
				return `Pinned to ${poolChoice}, which is no longer in the pool; the remaining pools carry this provider with ${strategy}.`;
			}
			return `Pinned to ${pinnedLabel}, which leads every attempt; the other usable pools follow with ${strategy}.`;
		}
		if (strategyChoice !== '') {
			return `Follows the global pool with its own strategy (${PROXY_STRATEGY_LABELS[effectiveStrategy]}), and proxying is ${globalEnabled ? 'on' : 'off'}.`;
		}
		return `Follows the global proxy setting (${globalEnabled ? `on, ${PROXY_STRATEGY_LABELS[effectiveStrategy]}` : 'off'}).`;
	}

	/** Builds the entry from the two controls, or null when nothing is left to store. */
	function buildEntry(
		poolID: string,
		strategy: '' | ProviderProxyStrategy
	): ProviderProxyPatch | null {
		const entry: ProviderProxyPatch = {};
		if (poolID !== '') entry.pool_id = poolID;
		if (strategy !== '') entry.strategy = strategy;
		return Object.keys(entry).length === 0 ? null : entry;
	}

	async function write(next: ProviderProxyPatch | null, done: string): Promise<boolean> {
		message = null;
		saving = true;
		const result = await patchProviderProxies(providerId, next);
		saving = false;
		if (!result.ok) {
			message = result.error.message;
			return false;
		}
		applyNetwork(result.data.network);
		message = done;
		return true;
	}

	function choosePool(value: string): void {
		// The select has already moved, so the card takes the new value now and gives it back if the
		// gateway refuses. An empty value is Global, which is the absent `pool_id`, never `''`: an entry
		// that set neither field is the one shape the gateway refuses by name.
		poolChoice = value;
		const label = pools.find((pool) => pool.id === value)?.label ?? value;
		const done =
			value === ''
				? 'Back on the global proxy setting.'
				: value === PROXY_POOL_NONE
					? 'This provider now dials direct.'
					: `This provider now uses ${label} first.`;
		void write(buildEntry(value, strategyChoice), done).then((ok) => {
			if (!ok) syncControls();
		});
	}

	function chooseStrategy(raw: string): void {
		// The select offers the empty default and the engine's own closed set, so anything else is a value
		// the control could not have produced; it reads as the default rather than entering the state.
		const value: '' | ProviderProxyStrategy =
			raw === 'fallback' || raw === 'round_robin' ? raw : '';
		strategyChoice = value;
		// When the strategy was the last thing stored, following the global one deletes the entry: the
		// message names that outcome rather than reporting a strategy that is no longer this provider's.
		const next = buildEntry(poolChoice, value);
		void write(
			next,
			next === null ? 'Back on the global proxy setting.' : 'Pool strategy saved.'
		).then((ok) => {
			if (!ok) syncControls();
		});
	}

	const fieldClass =
		'min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm';
</script>

<div class="flex flex-col gap-3">
	{#if loading}
		<p class="text-sm text-[var(--color-text-muted)]" role="status">Loading the proxy binding…</p>
	{:else if error}
		<p class="text-sm text-[var(--color-text-muted)]" role="status">
			The proxy binding could not be read: {error}
		</p>
	{:else}
		<div class="flex flex-col gap-1 text-sm">
			<label for="provider-proxy-pool">Proxy pool</label>
			<select
				id="provider-proxy-pool"
				class={fieldClass}
				value={poolChoice}
				disabled={saving}
				onchange={(event) => choosePool(event.currentTarget.value)}
			>
				{#each poolOptions as option (option.value)}
					<option value={option.value}>{option.label}</option>
				{/each}
			</select>
		</div>

		<div class="flex flex-col gap-1 text-sm">
			<label for="provider-proxy-strategy">Pool strategy</label>
			<select
				id="provider-proxy-strategy"
				class={fieldClass}
				value={strategyChoice}
				disabled={saving}
				onchange={(event) => chooseStrategy(event.currentTarget.value)}
			>
				<option value="">Follow the global strategy</option>
				{#each PROXY_STRATEGIES as strategy (strategy)}
					<option value={strategy}>{PROXY_STRATEGY_LABELS[strategy]}</option>
				{/each}
			</select>
		</div>

		<p class="text-xs text-[var(--color-text-muted)]">{summary}</p>
		{#if poolNotice}
			<p class="text-sm text-[var(--color-text-muted)]" role="status">
				The pool list could not be read: {poolNotice}
			</p>
		{/if}
		{#if message}
			<p class="text-sm text-[var(--color-text-muted)]" role="status">{message}</p>
		{/if}
	{/if}
</div>

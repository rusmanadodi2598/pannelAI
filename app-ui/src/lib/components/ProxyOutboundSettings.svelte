<script lang="ts">
	// The global outbound proxy settings (docs/SPEC-UI/001-SPEC-UI.md §6.9, §6.13).
	//
	// These keys decide the path every upstream call takes. With the pool engine
	// (docs/PORT/008-PORT-PROXY-ENGINE.md D1/D10) the pool above IS the route while proxying is on:
	// each call walks its rows in the strategy's order and the URL below is the last-resort attempt
	// after them, tried only when every pool candidate failed to connect. That ordering is stated on
	// the card rather than left for an operator to work out, because a URL that silently outranks a
	// tested pool, or a pool an operator adds rows to without learning they now carry traffic, is
	// the confusion this screen would otherwise cause.
	//
	// §6.9 also asks for the deferred per-endpoint binding to be named, and for the per-provider binding
	// to be pointed at: SPEC-API §7.11 defers the per-endpoint column, while a provider's own screen
	// (PORT 009) can pin that provider to one pool or to none. The card says both, so nobody hunts for a
	// control that does not exist or misses the one that does.
	//
	// §6.13 gives the Settings screen's Network tab a link to this surface rather than a second copy
	// of the form, so this is the one editor for these keys. It loads its own document, which keeps
	// the pool screen about the pool: a settings failure cannot stop the table from rendering.
	//
	// `outbound_no_proxy` stays a comma-separated string end to end, because that is what the API
	// sends and accepts; the schema normalizes it (trim, lowercase, dedupe, drop empty) without
	// changing shape.
	import { onMount } from 'svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { CONTROL_ICONS } from '$lib/icons';
	import { fetchSettings, patchNetworkSettings } from '$lib/api/settings';
	import { registerDirtyForm } from '$lib/dirty-guard';
	import {
		PROXY_STRATEGIES,
		PROXY_STRATEGY_LABELS,
		schemaNetworkSettingsForm,
		settingsGroupDirty,
		type NetworkSettingsForm
	} from '$lib/schemas/settings';

	const SaveIcon = CONTROL_ICONS.save.icon;

	let server = $state<NetworkSettingsForm | null>(null);
	let draft = $state<NetworkSettingsForm>({
		outbound_proxy_enabled: false,
		outbound_proxy_url: '',
		outbound_no_proxy: '',
		outbound_proxy_strategy: 'fallback'
	});
	let loading = $state(true);
	let error = $state<string | null>(null);
	let saving = $state(false);
	/** A refusal from the schema or the server, announced assertively because it blocked the save. */
	let refusal = $state<string | null>(null);
	let saved = $state(false);

	// This card owns four keys and nothing else. `provider_proxies` belongs to the provider screen's own
	// proxy card (docs/PORT/009-PORT-PROVIDER-PROXY.md D1), and carrying it here would put it in a strict
	// form that refuses it and in a PATCH body this card has no business writing.
	const ownKeys = (group: NetworkSettingsForm & Record<string, unknown>): NetworkSettingsForm => ({
		outbound_proxy_enabled: group.outbound_proxy_enabled,
		outbound_proxy_url: group.outbound_proxy_url,
		outbound_no_proxy: group.outbound_no_proxy,
		outbound_proxy_strategy: group.outbound_proxy_strategy
	});

	const dirty = $derived(settingsGroupDirty(server, draft));

	// §8.4.4: the shared guard asks before a navigation takes this draft away. Until the stored document
	// has been read there is nothing to be dirty against, so the loading window is not a draft.
	$effect(() => registerDirtyForm(() => server !== null && dirty));

	// A stored document with proxying on and no URL. That is a valid pool-only deployment (D1), so it
	// is stated as what will happen rather than alarmed over: the pool routes, and an empty pool
	// dials direct. Derived from what was read, not from the draft, so flipping the switch does not
	// claim the stored document is already in this state.
	const poolOnlyStoredState = $derived(
		server !== null && server.outbound_proxy_enabled && server.outbound_proxy_url === ''
	);

	onMount(() => void load());

	async function load(): Promise<void> {
		loading = true;
		const result = await fetchSettings();
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		server = ownKeys(result.data.network);
		draft = { ...server };
	}

	async function save(): Promise<void> {
		refusal = null;
		saved = false;
		const parsed = schemaNetworkSettingsForm.safeParse(draft);
		if (!parsed.success) {
			refusal = parsed.error.issues[0]?.message ?? 'Check these values.';
			return;
		}

		saving = true;
		const result = await patchNetworkSettings(parsed.data);
		saving = false;

		if (!result.ok) {
			refusal = result.error.message;
			return;
		}

		server = ownKeys(result.data.network);
		draft = { ...server };
		saved = true;
	}

	function discard(): void {
		if (server !== null) draft = { ...server };
		refusal = null;
		saved = false;
	}

	const fieldClass =
		'min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm';
</script>

<div class="flex flex-col gap-4 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4">
	<h2 class="text-sm font-semibold">Outbound proxy</h2>
	<p class="text-sm text-[var(--color-text-muted)]">
		Global, and the setting the gateway actually routes with, unless a provider has its own binding
		on its provider screen. While proxying is on, every upstream call walks the pool above in the
		strategy's order, unless its host is in the bypass list below. The URL field is the last resort:
		it is tried after the pool has failed to connect, never before a pool candidate. Per-endpoint
		binding is still deferred.
	</p>

	{#if loading}
		<StateMessage kind="loading" title="Loading the outbound proxy settings" />
	{:else if error && server === null}
		<StateMessage
			kind="error"
			title="The outbound proxy settings could not be loaded"
			description={error}
		>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => void load()}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else}
		{#if error}
			<p role="alert" class="text-sm text-[var(--color-danger)]">{error}</p>
		{/if}

		{#if poolOnlyStoredState}
			<p role="status" class="text-sm text-[var(--color-text-muted)]">
				Proxying is on with no URL, so the pool above is the whole route. If the pool has no rows
				yet, upstream calls go direct until you add one.
			</p>
		{/if}

		<label class="flex items-start gap-3 text-sm">
			<input type="checkbox" class="mt-1 size-4" bind:checked={draft.outbound_proxy_enabled} />
			<span>
				Route upstream calls through the proxy pool
				<span class="block text-xs text-[var(--color-text-muted)]">
					While this is on, the pool's rows carry traffic in the strategy's order. Turning it off
					sends every call direct and leaves the pool stored but unused.
				</span>
			</span>
		</label>

		<div class="flex flex-col gap-1 text-sm">
			<label for="outbound-proxy-strategy">Pool strategy</label>
			<select
				id="outbound-proxy-strategy"
				bind:value={draft.outbound_proxy_strategy}
				aria-describedby="outbound-proxy-strategy-help"
				class={fieldClass}
			>
				{#each PROXY_STRATEGIES as strategy (strategy)}
					<option value={strategy}>{PROXY_STRATEGY_LABELS[strategy]}</option>
				{/each}
			</select>
			<span id="outbound-proxy-strategy-help" class="text-xs text-[var(--color-text-muted)]">
				Fallback tries the rows in the order you saved them and sits a row out for two minutes after
				its connection fails. Round robin starts each request at the next row in line. Either way, a
				row whose connection fails hands the call to the next candidate, and the URL below is tried
				after the pool.
			</span>
		</div>

		<div class="flex flex-col gap-1 text-sm">
			<label for="outbound-proxy-url">Last-resort proxy URL</label>
			<input
				id="outbound-proxy-url"
				type="text"
				bind:value={draft.outbound_proxy_url}
				placeholder="http://proxy.example.com:8080"
				aria-describedby="outbound-proxy-url-help"
				class={fieldClass}
			/>
			<span id="outbound-proxy-url-help" class="text-xs text-[var(--color-text-muted)]">
				An absolute http or https URL, credentials included if the proxy needs them. Optional: with
				no URL, the pool alone carries traffic.
			</span>
		</div>

		<div class="flex flex-col gap-1 text-sm">
			<label for="outbound-no-proxy">Bypass the proxy for these hosts</label>
			<input
				id="outbound-no-proxy"
				type="text"
				bind:value={draft.outbound_no_proxy}
				placeholder="localhost,10.0.0.0/8"
				aria-describedby="outbound-no-proxy-help"
				class={fieldClass}
			/>
			<span id="outbound-no-proxy-help" class="text-xs text-[var(--color-text-muted)]">
				Comma-separated hosts or domain suffixes. A single * exempts everything. Duplicates and
				empty entries are removed on save.
			</span>
		</div>

		<div class="flex flex-wrap items-center gap-3">
			<button
				type="button"
				disabled={saving}
				class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-3 text-sm font-medium text-[var(--color-accent-text)] disabled:opacity-60"
				onclick={() => void save()}
			>
				<SaveIcon class="size-4" aria-hidden="true" />
				Save outbound settings
			</button>
			{#if dirty}
				<button type="button" class="min-h-11 underline" onclick={discard}>Discard changes</button>
				<span class="text-sm font-medium text-[var(--color-text-muted)]">Unsaved changes</span>
			{/if}
			{#if saved}
				<p class="text-sm text-[var(--color-text-muted)]" role="status">
					Saved. The next outbound call uses this path.
				</p>
			{/if}
		</div>

		{#if refusal}
			<p role="alert" class="text-sm text-[var(--color-danger)]">{refusal}</p>
		{/if}
	{/if}
</div>

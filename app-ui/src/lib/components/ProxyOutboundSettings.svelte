<script lang="ts">
	// The global outbound proxy settings (docs/SPEC-UI/001-SPEC-UI.md §6.9, §6.13).
	//
	// These three keys decide the path every upstream call takes, and they are the only proxy setting
	// the gateway routes with: the pool above holds tested candidates, and a candidate does not become
	// the outbound path until its address is set here. That is stated on the card rather than left for
	// an operator to work out, because adding a row to a pool and finding traffic still going direct
	// is the failure this screen would otherwise cause.
	//
	// §6.9 also asks for the deferred per-endpoint binding to be named. SPEC-API §7.11 assigns one
	// proxy globally in v1, so no control for it ships and the card says so instead of leaving a
	// reader to hunt for one.
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
	import { fetchSettings, patchNetworkSettings } from '$lib/api/settings';
	import { registerDirtyForm } from '$lib/dirty-guard';
	import {
		schemaNetworkSettingsForm,
		settingsGroupDirty,
		type NetworkSettingsForm
	} from '$lib/schemas/settings';

	let server = $state<NetworkSettingsForm | null>(null);
	let draft = $state<NetworkSettingsForm>({
		outbound_proxy_enabled: false,
		outbound_proxy_url: '',
		outbound_no_proxy: ''
	});
	let loading = $state(true);
	let error = $state<string | null>(null);
	let saving = $state(false);
	/** A refusal from the schema or the server, announced assertively because it blocked the save. */
	let refusal = $state<string | null>(null);
	let saved = $state(false);

	const dirty = $derived(settingsGroupDirty(server, draft));

	// §8.4.4: the shared guard asks before a navigation takes this draft away. Until the stored document
	// has been read there is nothing to be dirty against, so the loading window is not a draft.
	$effect(() => registerDirtyForm(() => server !== null && dirty));

	// A stored document with proxying on and no URL. The API accepts it and the egress path then dials
	// direct, so the panel cannot prevent a state it did not write; it states the state instead, with
	// both ways out (R-27). Derived from what was read, not from the draft, so flipping the switch does
	// not claim the stored document is already broken.
	const brokenStoredState = $derived(
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
		server = { ...result.data.network };
		draft = { ...result.data.network };
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

		server = { ...result.data.network };
		draft = { ...result.data.network };
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
		Global, and the setting the gateway actually routes with. Every upstream call the gateway makes
		goes through this URL, whichever endpoint it is for, unless its host is in the bypass list
		below. A candidate in the pool above is a stored, tested address; it does not carry traffic
		until you put it here. One proxy applies to everything in v1, so there is no per-endpoint
		control.
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

		{#if brokenStoredState}
			<p role="alert" class="text-sm text-[var(--color-danger)]">
				Proxying is on with no URL, so upstream calls go direct rather than through a proxy. Set a
				URL below, or turn the switch off.
			</p>
		{/if}

		<label class="flex items-start gap-3 text-sm">
			<input type="checkbox" class="mt-1 size-4" bind:checked={draft.outbound_proxy_enabled} />
			<span>
				Send upstream calls through a proxy
				<span class="block text-xs text-[var(--color-text-muted)]">
					When this is on, a URL is required. The panel will not save the switch on its own, because
					calls would then go direct while the setting reads as proxied.
				</span>
			</span>
		</label>

		<div class="flex flex-col gap-1 text-sm">
			<label for="outbound-proxy-url">Outbound proxy URL</label>
			<input
				id="outbound-proxy-url"
				type="text"
				bind:value={draft.outbound_proxy_url}
				placeholder="http://proxy.example.com:8080"
				aria-describedby="outbound-proxy-url-help"
				class={fieldClass}
			/>
			<span id="outbound-proxy-url-help" class="text-xs text-[var(--color-text-muted)]">
				An absolute http or https URL, credentials included if the proxy needs them.
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
				class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-3 text-sm font-medium text-[var(--color-accent-text)] disabled:opacity-60"
				onclick={() => void save()}>Save outbound settings</button
			>
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

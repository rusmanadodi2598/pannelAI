<script lang="ts">
	// The Connections section of the provider detail screen (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The section is the reference's own block (`providers/[id]/page.js:1508-1741`): a heading, the
	// rotation switch, the action that adds a connection, and the list. It is its own component because
	// both kinds of provider screen render it and the two put it in different places: the registry
	// screen leads with its facts and its model sections, while a custom node leads with this, because a
	// node with no connection has nothing to route with yet.
	//
	// The add action belongs to the page rather than here: a key added from the node's own details card
	// opens the same dialog, so the page owns that state and hands this section the button and the notice
	// that reports the last add.
	//
	// The rotation switch is the reference's per-provider control (`ConnectionsCard.js:405-427`) and it
	// writes a settings key rather than a provider field, so this section reads the settings document for
	// it. The override map is one value (§7.14): a write sends the whole map back, and a provider with no
	// entry inherits the global default, which is what the switch reports while it is off.
	import { resolve } from '$app/paths';
	import { untrack } from 'svelte';
	import { fetchSettings, patchProviderStrategy } from '$lib/api/settings';
	import ProviderEndpoints from '$lib/components/ProviderEndpoints.svelte';
	import { REQUIRES_KEY_AUTH_TYPES } from '$lib/schemas/endpoint-write';
	import {
		CREDENTIAL_ROTATION_LABELS,
		type CredentialRotation,
		type ProviderStrategy
	} from '$lib/schemas/settings';
	import type { ProviderDetail } from '$lib/schemas/provider';

	let {
		provider,
		token = 0,
		notice = null,
		onaddkey
	}: {
		provider: ProviderDetail;
		/** Bumped by the page when a key was added, so the table below re-reads. */
		token?: number;
		/** The last add's outcome, reported by the page that owns the dialog. */
		notice?: string | null;
		onaddkey: () => void;
	} = $props();

	// A key is only meaningful where the provider's own auth type takes one. For an OAuth provider the
	// dialog would ask for a credential the provider does not use, and the path that fits is the endpoint
	// form, which carries the auth type with it.
	const offersKeys = $derived(REQUIRES_KEY_AUTH_TYPES.has(provider.auth_type));

	// The override map as last read, kept whole so a write returns it whole.
	let overrides = $state<Record<string, ProviderStrategy>>({});
	let globalStrategy = $state<CredentialRotation>('fill-first');
	let globalSticky = $state(1);
	// The switch's own state rather than a derivation of the map: the browser flips the box before the
	// write is answered, and a refused write must put it back instead of leaving a switch that claims a
	// change the gateway never stored.
	let rotationOn = $state(false);
	let rotationLoading = $state(true);
	let rotationError = $state<string | null>(null);
	let rotationMessage = $state<string | null>(null);
	let saving = $state(false);

	// This provider's own entry, or null while it inherits the global default.
	const override = $derived(overrides[provider.id] ?? null);
	// What the sticky box shows: the provider's own limit when it set one, the effective global one
	// otherwise, because that is the value a rotation would use.
	const stickyValue = $derived(override?.sticky_limit ?? globalSticky);

	$effect(() => {
		// The id is read here so a route swap re-reads the map, and handed on so a slower earlier read
		// cannot overwrite a newer one.
		const id = provider.id;
		untrack(() => void loadRotation(id));
	});

	async function loadRotation(id: string): Promise<void> {
		rotationLoading = true;
		const result = await fetchSettings();
		rotationLoading = false;
		if (id !== provider.id) return;
		if (!result.ok) {
			rotationError = result.error.message;
			return;
		}
		rotationError = null;
		applyRotation(result.data.routing);
		overrides = { ...result.data.routing.provider_strategies };
		syncSwitch();
	}

	function applyRotation(routing: {
		fallback_strategy: CredentialRotation;
		sticky_limit: number;
	}): void {
		globalStrategy = routing.fallback_strategy;
		globalSticky = routing.sticky_limit;
	}

	/** Points the switch at what is stored for this provider. */
	function syncSwitch(): void {
		rotationOn = overrides[provider.id]?.fallback_strategy === 'round-robin';
	}

	async function writeRotation(next: ProviderStrategy | null, done: string): Promise<boolean> {
		rotationMessage = null;
		saving = true;
		const result = await patchProviderStrategy(provider.id, next);
		saving = false;
		if (!result.ok) {
			rotationMessage = result.error.message;
			return false;
		}
		overrides = { ...result.data.routing.provider_strategies };
		applyRotation(result.data.routing);
		syncSwitch();
		rotationMessage = done;
		return true;
	}

	function toggleRotation(enabled: boolean): void {
		// The box has already flipped, so the switch takes the new value now and gives it back if the
		// gateway refuses. Turning it off deletes the entry, which is how a provider returns to the
		// global default; an empty entry would be configuration the gateway refuses by name.
		rotationOn = enabled;
		void writeRotation(
			enabled ? { fallback_strategy: 'round-robin' } : null,
			enabled ? 'This provider now rotates its credentials.' : 'Back on the global default.'
		).then((ok) => {
			if (!ok) rotationOn = !enabled;
		});
	}

	function changeSticky(raw: string): void {
		const value = Number(raw);
		if (!Number.isInteger(value) || value < 1) {
			rotationMessage = 'The sticky limit must be a whole number of requests, at least 1.';
			return;
		}
		void writeRotation(
			{ fallback_strategy: 'round-robin', sticky_limit: value },
			'Sticky limit saved.'
		);
	}

	const fieldClass =
		'min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm';
</script>

<div class="flex flex-col gap-3">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<h2 class="text-base font-medium">Connections</h2>
		<div class="flex flex-wrap items-center gap-3">
			<div class="flex flex-wrap items-center gap-2">
				<label for="provider-round-robin" class="text-xs text-[var(--color-text-muted)]">
					Round Robin
				</label>
				<input
					id="provider-round-robin"
					type="checkbox"
					class="size-4"
					checked={rotationOn}
					disabled={rotationLoading || saving}
					onchange={(event) => toggleRotation(event.currentTarget.checked)}
				/>
				{#if rotationOn}
					<label for="provider-sticky" class="text-xs text-[var(--color-text-muted)]">
						Sticky:
					</label>
					<input
						id="provider-sticky"
						type="number"
						class="w-20 {fieldClass}"
						value={stickyValue}
						disabled={saving}
						onchange={(event) => changeSticky(event.currentTarget.value)}
					/>
				{/if}
			</div>
			{#if offersKeys}
				<button
					type="button"
					class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3"
					onclick={onaddkey}>Add API Key</button
				>
			{:else}
				<a
					href={resolve(`/endpoint-keys?provider=${encodeURIComponent(provider.id)}`)}
					class="min-h-11 content-center underline">Add a connection</a
				>
			{/if}
		</div>
	</div>
	{#if rotationError}
		<p class="text-sm text-[var(--color-text-muted)]" role="status">
			The rotation setting could not be read: {rotationError}
		</p>
	{:else if !rotationLoading}
		<p class="text-xs text-[var(--color-text-muted)]">
			{#if override === null}
				Credentials follow the global default ({CREDENTIAL_ROTATION_LABELS[globalStrategy]}).
			{:else}
				This provider overrides the global default ({CREDENTIAL_ROTATION_LABELS[globalStrategy]}).
			{/if}
		</p>
	{/if}
	{#if rotationMessage}
		<p class="text-sm text-[var(--color-text-muted)]" role="status">{rotationMessage}</p>
	{/if}
	{#if notice}
		<p class="text-sm" role="status">{notice}</p>
	{/if}
	<ProviderEndpoints providerId={provider.id} {token} />
</div>

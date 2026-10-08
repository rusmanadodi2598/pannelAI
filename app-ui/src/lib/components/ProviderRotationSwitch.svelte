<script lang="ts">
	// The Connections section's credential rotation switch (docs/SPEC-UI/001-SPEC-UI.md §6.3,
	// docs/SPEC-API/001-SPEC-API.md §7.14).
	//
	// This is the reference's per-provider control (`ConnectionsCard.js`) and it writes a settings
	// key rather than a provider field, so this component reads the settings document for it. The override
	// map is one value (§7.14): a write sends the whole map back, and a provider with no entry inherits the
	// global default, which is what the switch reports while it is off.
	//
	// It is its own component because it is a self-contained control: its own read, its own write, and the
	// lines that report both. The section around it only decides where it sits.
	import { untrack } from 'svelte';
	import { fetchSettings, patchProviderStrategy } from '$lib/api/settings';
	import {
		CREDENTIAL_ROTATION_LABELS,
		type CredentialRotation,
		type ProviderStrategy
	} from '$lib/schemas/settings';

	let { providerId }: { providerId: string } = $props();

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
	const override = $derived(overrides[providerId] ?? null);
	// What the sticky box shows: the provider's own limit when it set one, the effective global one
	// otherwise, because that is the value a rotation would use.
	const stickyValue = $derived(override?.sticky_limit ?? globalSticky);

	$effect(() => {
		// The id is read here so a route swap re-reads the map, and handed on so a slower earlier read
		// cannot overwrite a newer one.
		const id = providerId;
		untrack(() => void loadRotation(id));
	});

	async function loadRotation(id: string): Promise<void> {
		rotationLoading = true;
		const result = await fetchSettings();
		rotationLoading = false;
		if (id !== providerId) return;
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
		rotationOn = overrides[providerId]?.fallback_strategy === 'round-robin';
	}

	async function writeRotation(next: ProviderStrategy | null, done: string): Promise<boolean> {
		rotationMessage = null;
		saving = true;
		const result = await patchProviderStrategy(providerId, next);
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
			rotationMessage = 'Sticky limit needs a whole number, at least 1.';
			return;
		}
		void writeRotation(
			{ fallback_strategy: 'round-robin', sticky_limit: value },
			'Sticky limit saved.'
		);
	}

	const fieldClass =
		'min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm';
	// The switch reads as a control the same height and border as the section's action buttons, so it no
	// longer sits as a bare checkbox beside a full-height one. A `role="switch"` button is the accessible
	// equivalent of the checkbox it replaces: the on/off state is announced, and one click toggles it.
	const trackClass =
		'relative inline-flex min-h-11 items-center rounded-[var(--radius-sm)] border border-[var(--color-border)] px-2 text-sm disabled:opacity-50';
</script>

<div class="flex flex-col gap-1">
	<div class="flex flex-wrap items-center gap-2">
		<button
			type="button"
			class="{trackClass} {rotationOn
				? 'border-[var(--color-accent)] text-[var(--color-text)]'
				: 'text-[var(--color-text-muted)]'}"
			role="switch"
			aria-checked={rotationOn}
			disabled={rotationLoading || saving}
			onclick={() => toggleRotation(!rotationOn)}
		>
			<span class="me-2">Round Robin</span>
			<span class="flex w-9 items-center" aria-hidden="true">
				<span
					class="relative inline-block h-5 w-9 rounded-full transition-colors {rotationOn
						? 'bg-[var(--color-accent)]'
						: 'bg-[var(--color-surface-2)] border border-[var(--color-border)]'}"
				>
					<span
						class="absolute top-0.5 size-4 rounded-full bg-[var(--color-surface)] transition-all {rotationOn
							? 'left-[1.125rem]'
							: 'left-0.5'}"
					></span>
				</span>
			</span>
		</button>
		{#if rotationOn}
			<label for="provider-sticky" class="text-xs text-[var(--color-text-muted)]">Sticky:</label>
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
	{#if rotationError}
		<p class="text-sm text-[var(--color-text-muted)]" role="status">
			The rotation setting could not be read: {rotationError}
		</p>
	{:else if !rotationLoading}
		{#if override === null}
			<p class="text-xs text-[var(--color-text-muted)]">
				Credentials follow the global default ({CREDENTIAL_ROTATION_LABELS[globalStrategy]}).
			</p>
		{:else}
			<p class="text-xs text-[var(--color-text-muted)]">
				This provider overrides the global default ({CREDENTIAL_ROTATION_LABELS[globalStrategy]}).
			</p>
		{/if}
	{/if}
	{#if rotationMessage}
		<p class="text-sm text-[var(--color-text-muted)]" role="status">{rotationMessage}</p>
	{/if}
</div>

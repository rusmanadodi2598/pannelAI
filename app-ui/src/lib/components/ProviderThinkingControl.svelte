<script lang="ts">
	// The provider screen's reasoning picker (docs/SPEC-API/001-SPEC-API.md §7.14, §7.15;
	// docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// This is the reference's per-provider control (`providers/[id]/page.js:1748-1766`) in our
	// settings-map shape: it writes one provider's entry of `reasoning.provider_thinking`, so it reads
	// and writes the settings document through the screen's store. The map is one value (§7.14): a write
	// sends the whole map back, and a provider with no entry follows the reasoning setting each request
	// carries.
	//
	// The options are the levels the provider's own models accept, which is the union the server projects
	// onto the provider detail (§7.14, the reference's `providerThinkingLevels`): a level no model accepts
	// would be a control that answers an upstream error instead of a choice. `Auto` is the picker's word
	// for the absent entry and deletes it, which is the reference's own rule (`saveThinkingConfig`,
	// page.js:419-436). The select carries the reference's own title, because the suffix it warns about
	// is appended by the two model tables below rather than by this control.
	//
	// A stored mode the current model set no longer accepts stays on the list rather than falling back to
	// Auto: a select that hid it would misreport a setting the gateway still applies, and the operator
	// could not clear it from here.
	import {
		THINKING_AUTO,
		THINKING_MODES,
		thinkingModeLabel,
		type ThinkingMode
	} from '$lib/schemas/settings';
	import type { ProviderThinkingStore } from '$lib/stores/provider-thinking.svelte';

	let {
		providerId,
		levels,
		thinking
	}: {
		providerId: string;
		levels: readonly string[] | undefined;
		thinking: ProviderThinkingStore;
	} = $props();

	// The select's own state while a write is in flight: the browser moves a select before the write is
	// answered, and a refused write must put it back instead of leaving a control that claims a change the
	// gateway never stored. Null is "no write in flight", which is when the stored mode is the truth.
	let choice = $state<string | null>(null);

	const stored = $derived(thinking.modeFor(providerId));

	/** What the select shows: the in-flight choice, else the stored mode, else auto. */
	const shown = $derived(choice ?? (stored === '' ? THINKING_AUTO : stored));

	const options = $derived.by(() => {
		const list: string[] = [THINKING_AUTO, ...(levels ?? [])];
		if (stored !== '' && !list.includes(stored)) list.push(stored);
		return list;
	});

	/** The one value the write takes: a level, or null for the absent entry Auto stores. */
	function toMode(raw: string): ThinkingMode | null {
		if (raw === THINKING_AUTO) return null;
		// Every option is Auto or a level the registry declares, and the registry's sets are a subset of
		// the gateway's vocabulary; a value outside it is one the control could not have produced, so it
		// is refused rather than sent.
		return THINKING_MODES.find((mode) => mode === raw) ?? null;
	}

	async function choose(raw: string): Promise<void> {
		choice = raw;
		await thinking.setMode(providerId, toMode(raw));
		// Whether the write landed or not, the store is the truth again: a success moved the stored mode,
		// and a failure left it where it was, so the select reads what the gateway actually holds.
		choice = null;
	}

	/** The label an option renders with. A level no model accepts now says so, rather than reading as a
	 * choice the models would honour. */
	function optionLabel(option: string): string {
		const label = thinkingModeLabel(option);
		if (option !== THINKING_AUTO && !(levels ?? []).includes(option)) {
			return `${label} (not accepted now)`;
		}
		return label;
	}

	function describe(): string {
		if (shown === THINKING_AUTO) {
			return 'Follows the reasoning setting each request carries; a copied model name gains no suffix.';
		}
		return `Every request to this provider asks for the ${shown} level; a copied model name gains the (${shown}) suffix when that model accepts it.`;
	}

	const fieldClass =
		'min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm';
</script>

{#if levels !== undefined && levels.length > 0}
	<div class="flex flex-col gap-1 text-sm">
		<label for="provider-thinking-mode">Reasoning mode</label>
		{#if thinking.loading}
			<p class="text-[var(--color-text-muted)]" role="status">Loading the reasoning mode…</p>
		{:else if thinking.error}
			<p class="text-[var(--color-text-muted)]" role="status">
				The reasoning setting could not be read: {thinking.error}
			</p>
		{:else}
			<select
				id="provider-thinking-mode"
				class={fieldClass}
				value={shown}
				disabled={thinking.saving}
				title="Appends (level) suffix to copied model names"
				onchange={(event) => void choose(event.currentTarget.value)}
			>
				{#each options as option (option)}
					<option value={option}>{optionLabel(option)}</option>
				{/each}
			</select>
			<p class="text-xs text-[var(--color-text-muted)]">{describe()}</p>
			{#if thinking.outcome}
				<p class="text-sm text-[var(--color-text-muted)]" role="status">{thinking.outcome}</p>
			{/if}
		{/if}
	</div>
{/if}
